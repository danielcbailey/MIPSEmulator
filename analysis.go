package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
)

type VetTestCase struct {
	Successes       int
	Fails           int
	ErrorsFrequency map[int]int //the key is the error type of the runtime error, the value is the amount of that type
	TotalErrors     int
}

type VetSnapshot struct {
	TestCase string
	Snapshot EmulationResult
}

type VetSession struct {
	Assignment      string
	CorrectCount    int
	TotalCount      int
	TestCases       map[string]*VetTestCase
	FailedSnapshots []VetSnapshot
}

//evaluates the probability
func (v *VetSession) addVetFailedSnap(result EmulationResult, tc string) {
	//only records a fraction of the failed snapshots.
	//the probability is determined from how many other snapshots there are of the same test case
	//it exponentially decreases with more snapshots.
	num := 0
	for _, s := range v.FailedSnapshots {
		if s.TestCase == tc {
			num++
		}
	}

	if rand.Float64() > math.Pow(0.75, float64(num)) {
		//not capturing this failure
		return
	}

	v.FailedSnapshots = append(v.FailedSnapshots, VetSnapshot{
		TestCase: tc,
		Snapshot: result,
	})
}

func addVetErrors(errors []RuntimeError, vErrors map[int]int) map[int]int {
	for _, e := range errors {
		v, ok := vErrors[e.EType]
		if !ok {
			vErrors[e.EType] = 1
		} else {
			vErrors[e.EType] = v + 1
		}
	}

	return vErrors
}

func newVet(aName string) *VetSession {
	ret := new(VetSession)
	ret.TestCases = make(map[string]*VetTestCase)
	ret.Assignment = aName
	return ret
}

func (v *VetSession) displayResults() {
	avgErr := 0.0
	for _, val := range v.TestCases {
		avgErr += float64(val.TotalErrors)
	}
	avgErr /= float64(v.TotalCount)

	fmt.Println("\n+====[ VET RESULTS ]====+")
	fmt.Printf("Vet for %s.\n", v.Assignment)
	fmt.Printf("Summary:\n")
	fmt.Printf(" - Performed %d tests.\n", v.TotalCount)
	fmt.Printf(" - Of those, %d were successful (%.3f%% success rate).\n", v.CorrectCount, float64(v.CorrectCount)/float64(v.TotalCount)*100)
	fmt.Printf(" - For each evaluation, on average there were %.3f errors.\n", avgErr)

	fmt.Printf("\nTest Cases (%d) (Organized into categories; categories are not mutually exclusive):\n", len(v.TestCases))
	//category detection
	//format is as such: assignment-cat1-cat2-cat3-...-catn
	options := make(map[int]map[string]*VetTestCase)
	for k, v := range v.TestCases {
		categories := strings.Split(k, "-")
		for i := 1; len(categories) > i; i++ {
			c, ok := options[i]
			if !ok {
				c = make(map[string]*VetTestCase)
				options[i] = c
			}

			cv, ok := c[categories[i]]
			if !ok {
				cv = new(VetTestCase)
				cv.TotalErrors = 0
				cv.Fails = 0
				cv.Successes = 0
				cv.ErrorsFrequency = make(map[int]int)
				c[categories[i]] = cv
			}

			cv.Successes += v.Successes
			cv.Fails += v.Fails
			cv.TotalErrors += v.TotalErrors
			for ek, ev := range v.ErrorsFrequency {
				ec, ok := cv.ErrorsFrequency[ek]
				if !ok {
					cv.ErrorsFrequency[ek] = ev
				} else {
					ec += ev
					cv.ErrorsFrequency[ek] = ec
				}
			}
		}
	}

	for _, vi := range options {
		for kj, vj := range vi {
			fmt.Printf(" - %s: Successes: %d; Fails: %d; Error Count: %d\n", kj, vj.Successes, vj.Fails, vj.TotalErrors)
			for ek, ef := range vj.ErrorsFrequency {
				fmt.Printf("   + Error: %s; Count: %d (%.3f%%)\n", decodeErrorCode(ek), ef, float64(ef)/float64(vj.TotalErrors)*100)
			}
		}
		fmt.Println("")
	}
}

func displayGeneralResults(n, dimin, dimax, si int, avgdi float64, errors []RuntimeError, fName string) {
	fmt.Println("\n+====[ EMULATION RESULTS ]====+")
	fmt.Printf("Emulation of %s.\n", fName)
	fmt.Printf("Summary:\n")
	fmt.Printf(" - Performed %d tests.\n", n)
	fmt.Printf(" - %d SI; %5.2f average DI (min: %d, max: %d)\n", si, avgdi, dimin, dimax)

	if errors != nil {
		fmt.Printf(" - Total errors generated: %d\n", len(errors))
		fmt.Printf("\nAll errors:\n")
		for _, e := range errors {
			fmt.Printf(" - %s; %s\n", decodeErrorCode(e.EType), e.Message)
		}
	}
}
