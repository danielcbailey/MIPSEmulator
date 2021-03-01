package main

import (
	"fmt"
	"io/ioutil"
)

/**
 * Arguments are as such:
 * assemblyFile flags
 * Flags:
 * 		-v {assignment, valid options are P1}, example: "-v P1": Vets the assembly for the specified project
 * 			Vetting will perform 100,000 tests on the code and will report the success rate and where the code failed.
 * 		-d : Detailed op-code analysis.
 * 		-p : Performance analytics, reports branching information, dead code detection, and loop analysis.
 * 		-l {integer limit of DI}, example: "-l 250000": Changes runtime length limit from default 500,000 DI
 * 		-ts {starting address of text}, example: "-ts 0x4000": Changes the text start address from default 0x1000
 * 		-ds {starting address of data}, example: "-ds 0x4000": Changes the data start address from default 0x8000
 * 		-m {number of threads}, example: "-m 4": Changes number of threads from default 1
 */
func main() {
	//testing: only using default file of test.asm
	asmFile := "test.asm"
	b, e := ioutil.ReadFile(asmFile)
	if e != nil {
		fmt.Println("ERROR: Failed to open assembly file: " + e.Error())
		return
	}
	settings := AssemblySettings{
		TextStart: 0x1000,
		DataStart: 0x8000,
	}

	sysMem, lineMeta, numE := Assemble(string(b), settings)
	if numE != 0 {
		fmt.Printf("%d error(s) generated from assembler, not attempting emulation.\n", numE)
		return
	}

	vet := true
	numSamples := 1
	limit := 100000
	var vetSession *VetSession
	if vet {
		vetSession = newVet("Project 1") //TODO: change to fetch from arguments
		numSamples = 100000
	}

	var lastResult EmulationResult
	numInf := 0
	dimin := limit
	dimax := 0
	avgDI := 0.0
	var sysMemCopy SystemMemory
	for i := 0; numSamples > i; i++ {
		//creating a copy of the memory
		sysMemCopy = make(SystemMemory)
		for k, v := range sysMem {
			newPage := MemoryPage{
				startAddr:   v.startAddr,
				memory:      make([]uint32, len(v.memory)),
				initialized: make([]uint32, len(v.initialized)),
			}

			copy(newPage.memory, v.memory)
			copy(newPage.initialized, v.initialized)

			sysMemCopy[k] = newPage
		}

		//performing the emulation
		lastResult = Emulate(settings.TextStart, sysMemCopy, uint32(limit))

		avgDI += float64(lastResult.DI)
		if int(lastResult.DI) < dimin {
			dimin = int(lastResult.DI)
		}
		if int(lastResult.DI) > dimax {
			dimax = int(lastResult.DI)
		}

		//checking health of output
		if len(lastResult.Errors) > 0 && lastResult.Errors[len(lastResult.Errors)-1].EType == eRuntimeLimitExceeded {
			numInf++

			if numInf > 10 {
				//too many infinite loops
				fmt.Println("\n+====[ HALTED DUE TO TOO MANY INFINITE LOOPS ]===+")
				numSamples = i + 1
				break
			}
		}

		if vetSession != nil {
			vetSession.vetP1Interop(lastResult)
		}

		//updating user every 10%
		if numSamples > 10000 && i%(numSamples/10) == 0 {
			fmt.Printf("Progress: Completed %d%% (%d emulations)\n", i/(numSamples/100), i)
		}
	}
	eSlice := lastResult.Errors
	if numSamples > 1 {
		eSlice = nil
	}

	displayGeneralResults(numSamples, dimin, dimax, len(lineMeta), avgDI/float64(numSamples), eSlice, asmFile)

	if vetSession != nil {
		vetSession.displayResults()
	}
}
