package main

func (inst *instance) dispatchSoftwareInterrupt(iCode int) {
	switch iCode {
	case 582:
		inst.swi582()
		break
	case 583:
		inst.swi583()
		break
	case 598:
		inst.swi598()
		break
	case 599:
		inst.swi599()
		break
	}
}
