package uadp

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	dbus "github.com/godbus/dbus"
	accounts "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.accounts"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/procfs"
)

//go:generate dbusutil-gen em -type Uadp

var logger = log.NewLogger("daemon/Uadp")

const (
	dbusServiceName = "com.deepin.daemon.Uadp"
	dbusPath        = "/com/deepin/daemon/Uadp"
	dbusInterface   = dbusServiceName
)

const (
	allowedProcess = "/usr/lib/deepin-daemon/dde-session-daemon"

	UadpDataDir = ".local/share/uadp"
)

func (*Uadp) GetInterfaceName() string {
	return dbusInterface
}

type Uadp struct {
	service    *dbusutil.Service
	account    accounts.Accounts
	appDataMap map[uint32]map[string]map[string][]byte // 应用加密数据缓存
	fileNames  map[uint32]map[string]string            // 文件索引缓存

	secretMu sync.Mutex
	mu       sync.Mutex
}

func newUadp(service *dbusutil.Service) (*Uadp, error) {
	u := &Uadp{
		service:    service,
		account:    accounts.NewAccounts(service.Conn()),
		appDataMap: make(map[uint32]map[string]map[string][]byte),
		fileNames:  make(map[uint32]map[string]string),
	}
	return u, nil
}

// 加密用户存储的密钥并存储在文件中
func (u *Uadp) SetDataKey(sender dbus.Sender, exePath, keyName, dataKey, keyringKey string) *dbus.Error {
	_, err := u.verifyIdentity(sender)
	if err != nil {
		logger.Warning("failed to verify:", err)
		return dbusutil.ToError(err)
	}
	logger.Debug("invoker has been verified")
	uid, err := u.service.GetConnUID(string(sender))
	if err != nil {
		logger.Warning("failed to get uid:", err)
		return dbusutil.ToError(err)
	}
	err = u.setDataKey(uid, exePath, keyName, dataKey, keyringKey)
	if err != nil {
		logger.Warning("failed to encrypt key:", err)
		return dbusutil.ToError(err)
	}
	return nil
}

func (u *Uadp) setDataKey(uid uint32, exePath, keyName, dataKey, keyringKey string) error {
	encryptedKey, err := aesEncryptKey(dataKey, keyringKey)
	if err != nil {
		logger.Warning("failed to encryptKey by aes:", err)
		return err
	}
	u.secretMu.Lock()
	if u.appDataMap[uid] == nil {
		u.appDataMap[uid] = make(map[string]map[string][]byte)
	}
	if u.appDataMap[uid][exePath] == nil {
		u.appDataMap[uid][exePath] = make(map[string][]byte)
	}

	u.appDataMap[uid][exePath][keyName] = encryptedKey
	u.secretMu.Unlock()
	logger.Debug("get secret map:", u.appDataMap)

	err = u.updateDataFile(uid, exePath)
	if err != nil {
		logger.Warning("failed to updateDataFile:", err)
		return err
	}
	return nil
}

func aesEncryptKey(origin, key string) ([]byte, error) {
	origData := []byte(origin)
	k := []byte(key)

	// 分组秘钥
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	// 获取秘钥块的长度
	blockSize := block.BlockSize()
	// 补全码
	origData = pkcs7Padding(origData, blockSize)
	// 加密模式
	blockMode := cipher.NewCBCEncrypter(block, k[:blockSize])
	// 创建数组
	encrypted := make([]byte, len(origData))
	// 加密
	blockMode.CryptBlocks(encrypted, origData)
	return encrypted, nil
}

func pkcs7Padding(cipherText []byte, blockSize int) []byte {
	padding := blockSize - len(cipherText)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(cipherText, padText...)
}

func (u *Uadp) GetDataKey(sender dbus.Sender, exePath, keyName, keyringKey string) (dataKey string, busErr *dbus.Error) {
	_, err := u.verifyIdentity(sender)
	if err != nil {
		logger.Warning("failed to verify:", err)
		return "", dbusutil.ToError(err)
	}

	uid, err := u.service.GetConnUID(string(sender))
	if err != nil {
		logger.Warning("failed to get uid:", err)
		return "", dbusutil.ToError(err)
	}

	dataKey, err = u.getDataKey(uid, exePath, keyName, keyringKey)
	if err != nil {
		logger.Warning("failed to decrypt Key:", err)
		return "", dbusutil.ToError(err)
	}
	return dataKey, nil
}

func (u *Uadp) getDataKey(uid uint32, exePath, keyName, keyringKey string) (string, error) {
	encryptedKey := u.findKeyFromCacheOrFile(uid, exePath, keyName)
	if encryptedKey == nil {
		return "", errors.New("failed to find data used to be decrypted")
	}
	key, err := u.aesDecrypt(encryptedKey, keyringKey)
	if err != nil {
		logger.Warning("failed to aesDecrypt key:", err)
		return "", err
	}
	return key, nil
}

func (u *Uadp) aesDecrypt(encryptedKey []byte, key string) (string, error) {
	encryptedKeyByte := []byte(encryptedKey)
	k := []byte(key)

	// 分组秘钥
	block, err := aes.NewCipher(k)
	if err != nil {
		logger.Warning("failed to newCipher key:", err)
		return "", err
	}
	// 获取秘钥块的长度
	blockSize := block.BlockSize()
	// 加密模式
	blockMode := cipher.NewCBCDecrypter(block, k[:blockSize])
	// 创建数组
	decrypted := make([]byte, len(encryptedKeyByte))
	// 解密
	blockMode.CryptBlocks(decrypted, encryptedKeyByte)
	// 去补全码
	decrypted = pkcs7UnPadding(decrypted)
	return string(decrypted), nil
}

func pkcs7UnPadding(originData []byte) []byte {
	length := len(originData)
	unpadding := int(originData[length-1])
	return originData[:(length - unpadding)]
}

func (u *Uadp) findKeyFromCacheOrFile(uid uint32, exePath, keyName string) []byte {
	u.secretMu.Lock()
	if _, ok := u.appDataMap[uid]; !ok {
		u.appDataMap[uid] = make(map[string]map[string][]byte)
	}
	if _, ok := u.appDataMap[uid][exePath]; !ok {
		secretData, err := u.loadDataFromFile(uid, exePath)
		if err != nil {
			logger.Warning("failed to loadDataFromFile:", err)
			u.secretMu.Unlock()
			return nil
		}
		u.appDataMap[uid][exePath] = secretData
	}
	if value, ok := u.appDataMap[uid][exePath][keyName]; ok {
		u.secretMu.Unlock()
		return value
	}
	u.secretMu.Unlock()

	return nil
}

func (u *Uadp) loadDataFromFile(uid uint32, exePath string) (map[string][]byte, error) {
	fileName, err := u.getFileName(uid, exePath, false)
	if err != nil {
		logger.Warning("failed to get filename:", err)
		return nil, err
	}
	var secretData map[string][]byte
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		logger.Warning("cannot read data from file:", err)
		return nil, err
	}
	err = unmarshalGob(content, &secretData)
	if err != nil {
		logger.Warning(err)
		return nil, err
	}

	return secretData, nil
}

func (u *Uadp) updateDataFile(uid uint32, exePath string) error {
	secretData := u.appDataMap[uid][exePath]
	logger.Debug("begin to get file name")
	fileName, err := u.getFileName(uid, exePath, true)
	if err != nil {
		logger.Warning("failed to get filename:", err)
		return err
	}
	logger.Debug("update data file, get file name:", fileName)

	newFileName := fileName + "-1"
	content, err := marshalGob(secretData)
	if err != nil {
		logger.Warning(err)
		return err
	}
	err = writeFile(newFileName, content, 0600)
	if err != nil {
		logger.Warning(err)
		return err
	}

	err = os.Rename(newFileName, fileName)
	if err != nil {
		logger.Warning(err)
		return err
	}
	return nil
}

func (u *Uadp) getFileName(uid uint32, exePath string, createFileMap bool) (string, error) {
	homeDir, err := u.getHomeDir(fmt.Sprint(uid))
	if err != nil {
		return "", err
	}
	uadpDataFileDir := filepath.Join(homeDir, UadpDataDir)
	err = os.MkdirAll(uadpDataFileDir, 0755)
	if err != nil {
		logger.Warning(err)
		return "", err
	}
	var fileName string
	if _, ok := u.fileNames[uid]; !ok {
		u.fileNames[uid] = make(map[string]string)
	}
	fileName = u.fileNames[uid][exePath]
	var fileNames map[string]string

	if fileName == "" {
		fileMap := filepath.Join(uadpDataFileDir, "filemap")
		content, err := ioutil.ReadFile(fileMap)
		if err == nil {
			err = json.Unmarshal(content, &fileNames)
			if err != nil {
				logger.Warning(err)
				return "", err
			}
			u.mu.Lock()
			u.fileNames[uid] = fileNames
			fileName = u.fileNames[uid][exePath]
			u.mu.Unlock()
		}
		if fileName == "" {
			// 新增文件索引
			if createFileMap {
				fileName = fmt.Sprintf("%d", len(u.fileNames[uid]))
				u.mu.Lock()
				u.fileNames[uid][exePath] = fileName
				u.mu.Unlock()
				content, err := json.Marshal(u.fileNames[uid])
				if err != nil {
					logger.Warning(err)
					return "", err
				}
				err = writeFile(fileMap, content, 0600)
				if err != nil {
					logger.Warning(err)
					return "", err
				}
			} else {
				return "", errors.New("this file is not exist")
			}
		}
	}

	file := filepath.Join(uadpDataFileDir, fileName)
	return file, nil
}

func (u *Uadp) getHomeDir(uid string) (string, error) {
	userObjPath, err := u.account.FindUserById(0, uid)
	if err != nil {
		return "", err
	}
	user, err := accounts.NewUser(u.service.Conn(), dbus.ObjectPath(userObjPath))
	if err != nil {
		return "", err
	}
	homeDir, err := user.HomeDir().Get(0)
	if err != nil {
		return "", err
	}
	return homeDir, nil
}

func (u *Uadp) verifyIdentity(sender dbus.Sender) (bool, error) {
	pid, err := u.service.GetConnPID(string(sender))
	if err != nil {
		logger.Warning("failed to get PID:", err)
		return false, err
	}

	process := procfs.Process(pid)
	executablePath, err := process.Exe()
	if err != nil {
		logger.Warning("failed to get executablePath:", err)
		return false, err
	}

	if executablePath == allowedProcess {
		return true, nil
	}

	return false, errors.New("process is not allowed to access")
}

func writeFile(filename string, data []byte, perm os.FileMode) error {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		logger.Warning(err)
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	_, err = file.Write(data)
	if err != nil {
		logger.Warning(err)
		return err
	}

	err = file.Sync()
	if err != nil {
		logger.Warning(err)
		return err
	}
	return err
}

func unmarshalGob(content []byte, secretData interface{}) error {
	r := bytes.NewReader(content)
	dec := gob.NewDecoder(r)
	if err := dec.Decode(secretData); err != nil {
		logger.Warning("decode datas failed:", err)
		return err
	}
	return nil
}

func marshalGob(secretData map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(secretData)
	if err != nil {
		logger.Warning(err)
		return nil, err
	}

	return buf.Bytes(), nil
}
