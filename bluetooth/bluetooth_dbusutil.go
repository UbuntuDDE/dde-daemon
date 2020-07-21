package bluetooth

func (v *Bluetooth) setPropState(value uint32) (changed bool) {
	if v.State != value {
		v.State = value
		v.emitPropChangedState(value)
		return true
	}
	return false
}

func (v *Bluetooth) emitPropChangedState(value uint32) error {
	return v.service.EmitPropertyChanged(v, "State", value)
}
