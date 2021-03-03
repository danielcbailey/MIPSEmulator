package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"
)

type p1Rot int

const (
	p1Sol0Rot p1Rot = iota
	p1Sol90Rot
	p1Sol180Rot
	p1Sol270Rot
)

type Project1 struct {
	Reference        uint32    `json:"reference"`
	Candidates       [8]uint32 `json:"candidates"`
	SolutionOffset   uint32    `json:"solution"`
	SolutionRotation p1Rot     `json:"solutionRotation"`
	SolutionFlipped  bool      `json:"solutionFlipped"`
	ReportedOffset   uint32    `json:"reportedOffset"`
}

func (p *Project1) genSquare() uint32 {
	var t uint32
	for true {
		t = uint32(rand.Intn(65536))

		//testing for contiguous color, which is not allowed
		for i := 0; 8 > i; i++ {
			t1 := uint32(0x3 << ((i * 2) % 16))
			t2 := t & t1
			t3 := uint32(0x3 << ((i*2 + 2) % 16))
			t4 := (t & t3) >> 2
			if t2 == t4 {
				//reject sample
				break
			} else if i == 7 {
				//accept sample
				return t
			}
		}
	}

	panic("not possible")
}

func (p *Project1) testSolution(square uint32) bool {
	//again, this is purposefully inefficient to not give away too many hints
	//although, no matter how inefficient, some hints may be obtained by studying the code
	for i := 0; 4 > i; i++ {
		square = (square >> 4) | ((square & 0xF) << 12)

		if square == p.Reference {
			return true
		}
	}

	//flipping the square
	//purposefully making an inefficient algorithm to not give hints
	var buf [8]byte
	for i := range buf {
		buf[i] = byte((square >> (i * 2)) & 0x3)
	}
	square = 0
	for i := 0; 8 > i; i++ {
		square <<= 2
		square |= uint32(buf[(i+1)%8])
	}

	for i := 0; 4 > i; i++ {
		square = (square >> 4) | ((square & 0xF) << 12)

		if square == p.Reference {
			return true
		}
	}

	return false
}

func (p *Project1) genSolution() {
	p.SolutionOffset = uint32(4 * rand.Intn(8))
	p.SolutionFlipped = rand.Intn(2) == 0
	p.SolutionRotation = p1Rot(rand.Intn(4))

	//flipping is always first, then rotation
	sol := p.Reference
	if p.SolutionFlipped {
		//flipping the Reference
		//purposefully making an inefficient algorithm to not give hints
		var buf [8]byte
		for i := range buf {
			buf[i] = byte((sol >> (i * 2)) & 0x3)
		}
		sol = 0
		for i := 0; 8 > i; i++ {
			sol <<= 2
			sol |= uint32(buf[(i+1)%8])
		}
	}

	switch p.SolutionRotation {
	case p1Sol90Rot:
		sol = (sol >> 4) | ((sol & 0xF) << 12)
		break
	case p1Sol180Rot:
		sol = (sol >> 8) | ((sol & 0xFF) << 8)
		break
	case p1Sol270Rot:
		sol = (sol >> 12) | ((sol & 0xFFF) << 4)
		break
	}

	p.Candidates[p.SolutionOffset/4] = sol

	if sol == p.Reference {
		sol += 0 //something to get a breakpoint on
	}
}

func (inst *instance) swi582() {
	//memory address in register $1
	rand.Seed(time.Now().UnixNano())
	if !inst.regInitialized(1) {
		inst.reportError(eSoftwareInterruptParameter, "register $1 uninitialized for swi 582 call. $1 should hold the Reference memory pointer")
	}

	p := new(Project1)
	p.Reference = p.genSquare()
	p.genSolution()
	p.ReportedOffset = 0x12345678 //an arbitrary number to compare to if there was even an attempt at solving it

	a := inst.regs[1]
	inst.memWrite(a, p.Reference, 0xFFFFFFFF)
	inst.memWrite(a+4+p.SolutionOffset, p.Candidates[p.SolutionOffset/4], 0xFFFFFFFF)

	//generating dummy squares
	for i := 0; 8 > i; i++ {
		if int(p.SolutionOffset/4) == i {
			continue //already generated the solution
		}

		watchdog := 0

		for true {
			t := p.genSquare()
			if !p.testSolution(t) {
				p.Candidates[i] = t
				inst.memWrite(a+uint32(i)*4+4, t, 0xFFFFFFFF)
				break
			}
			watchdog++
			if watchdog > 1000 {
				watchdog = 0
				fmt.Println("Randomization watchdog intervened")
				rand.Seed(time.Now().UnixNano())
			}
		}
	}

	inst.swiContext = p
}

func (inst *instance) swi583() {
	//getting project info
	var p *Project1
	p, ok := inst.swiContext.(*Project1)
	if !ok {
		inst.reportError(eInvalidSoftwareInterrupt, "cannot use swi 583 with the previous swi call(s)")
		return
	}

	//offset in register $3
	if !inst.regInitialized(1) {
		inst.reportError(eSoftwareInterruptParameter, "register $3 uninitialized for swi 583 call. "+
			"$3 should hold the byte offset of the solution from the first candidate")
	}

	p.ReportedOffset = inst.regAccess(3)
	if p.ReportedOffset > 28 || p.ReportedOffset%4 != 0 {
		inst.reportError(eSoftwareInterruptParameterValue, "%h is an invalid solution for swi 583. Must be in [0, 28] and word aligned (multiple of four)")
		return
	}

	//storing solution
	inst.regWrite(6, p.SolutionOffset)
}

func (v *VetSession) vetP1Interop(result EmulationResult) {
	v.TotalCount++

	p, ok := result.SWIContext.(*Project1)
	if !ok {
		//fatal error, software interrupts not called for the vet case
		fmt.Println("FATAL: Software interrupt swi 582 not called for the P1 vet, terminating emulation..")
		os.Exit(1)
	}

	if p.ReportedOffset == 0x12345678 {
		//no guess was made
		result.Errors = append(result.Errors, RuntimeError{
			EType:   eNoAnswerReported,
			Message: "No call was made to swi 583 ",
		})
	}
	if p.ReportedOffset == p.SolutionOffset {
		//correct
		v.CorrectCount++
	}

	//create test case string
	rotStr := ""
	switch p.SolutionRotation {
	case p1Sol0Rot:
		rotStr = "0Rot"
		break
	case p1Sol90Rot:
		rotStr = "90Rot"
		break
	case p1Sol180Rot:
		rotStr = "180Rot"
		break
	case p1Sol270Rot:
		rotStr = "270Rot"
	}

	flipStr := "flipped"
	if !p.SolutionFlipped {
		flipStr = "notFlipped"
	}

	tCase := "P1-" + rotStr + "CW-" + flipStr + "-" + strconv.Itoa(int(p.SolutionOffset)) + "offset"

	tcs, ok := v.TestCases[tCase]
	if ok {
		ef := tcs.ErrorsFrequency
		addVetErrors(result.Errors, ef)
		v.TestCases[tCase].TotalErrors = tcs.TotalErrors + len(result.Errors)
		v.TestCases[tCase].ErrorsFrequency = ef
		if p.ReportedOffset == p.SolutionOffset {
			v.TestCases[tCase].Successes++
		} else {
			v.TestCases[tCase].Fails++
			v.addVetFailedSnap(result, tCase)
		}
	} else {
		ef := make(map[int]int)
		ef = addVetErrors(result.Errors, ef)
		v.TestCases[tCase] = new(VetTestCase)
		v.TestCases[tCase].ErrorsFrequency = ef
		v.TestCases[tCase].TotalErrors = len(result.Errors)
		if p.ReportedOffset == p.SolutionOffset {
			v.TestCases[tCase].Successes = 1
			v.TestCases[tCase].Fails = 0
		} else {
			v.TestCases[tCase].Successes = 0
			v.TestCases[tCase].Fails = 1
			v.addVetFailedSnap(result, tCase)
		}
	}
}
