package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

type p1Obscurity int
type p1Spacing int
type p1Geometry int

const (
	p1ObscurityNone p1Obscurity = iota
	p1ObscurityHorz
	p1ObscurityVert
	p1ObscurityBoth
)

const (
	p1SpacingNone p1Spacing = iota
	p1SpacingHorz
	p1SpacingVert
	p1SpacingBoth
)

const (
	p1GeometryNone p1Geometry = iota
	p1GeometryL
	p1GeometryT
)

type Project1Fa21 struct {
	TargetColor    uint32       `json:"target_color"`
	Solution       uint32       `json:"solution"`
	ReportedAnswer uint32       `json:"reported_answer"`
	Obscurity      p1Obscurity  `json:"obscurity"`
	Spacing        p1Spacing    `json:"spacing"`
	Geometry       p1Geometry   `json:"geometry"`
	HorzLineCount  int          `json:"horz_lines"`
	VertLineCount  int          `json:"vert_lines"`
	Pile           [1024]uint32 `json:"pile"`
	horzAllocs     uint64
	vertAllocs     uint64
	vLines         []int
	hLines         []int
	tlx            int
	tly            int
}

func (p *Project1Fa21) plotPoint(x, y, color int) {
	p.Pile[y*16+x/4] &= (0xFF << ((x % 4) * 8)) ^ 0xFFFFFFFF
	p.Pile[y*16+x/4] |= uint32(color) << ((x % 4) * 8)
}

func (p *Project1Fa21) getPoint(x, y int) uint32 {
	return (p.Pile[y*16+x/4] >> ((x % 4) * 8)) & 0xFF
}

func (p *Project1Fa21) drawHLine(x1, x2, y, color int) {
	for x := x1; x <= x2; x++ {
		p.plotPoint(x, y, color)
	}
}

func (p *Project1Fa21) drawVLine(x, y1, y2, color int) {
	for y := y1; y <= y2; y++ {
		p.plotPoint(x, y, color)
	}
}

func (p *Project1Fa21) checkHAlloc(y int) bool {
	return (p.horzAllocs>>y)&0x1 != 0 //returns 1 if already occupied
}

func (p *Project1Fa21) setHAlloc(y int) {
	p.horzAllocs |= 0x1 << y
}

func (p *Project1Fa21) checkVAlloc(x int) bool {
	return (p.vertAllocs>>x)&0x1 != 0 //returns 1 if already occupied
}

func (p *Project1Fa21) setVAlloc(x int) {
	p.vertAllocs |= 0x1 << x
}

func intAbs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func (p *Project1Fa21) generatePart(color int, isTarget bool) bool {
	width := rand.Intn(21) + 25
	height := rand.Intn(21) + 25

	targetVertLines := width / 12
	targetHorzLines := height / 12

	tlx := rand.Intn(62-width) + 1
	tly := rand.Intn(62-height) + 1

	hLines := make([]int, 0)
	vLines := make([]int, 0)

	//Attempting to generate horizontal lines
	for i := 0; targetHorzLines > i; i++ {
		for a := 0; 10 > a; a++ {
			//Makes 10 attempts to generate a line, will abort if 10 attempts is exceeded
			desiredY := rand.Intn(height) + tly

			//testing to see if it can place the line where it wants to
			if !p.checkHAlloc(desiredY) && !p.checkHAlloc(desiredY-1) && !p.checkHAlloc(desiredY+1) {
				//it can...
				p.setHAlloc(desiredY)
				//p.setHAlloc(desiredY - 1)
				//p.setHAlloc(desiredY + 1)
				p.drawHLine(tlx, tlx+width-1, desiredY, color)
				hLines = append(hLines, desiredY-tly)
				break // exiting the line drawing attempt loop
			}
		}
	}

	//Attempting to generate vertical lines
	for i := 0; targetVertLines > i; i++ {
		for a := 0; 10 > a; a++ {
			//Makes 10 attempts to generate a line, will abort if 10 attempts is exceeded
			desiredX := rand.Intn(width) + tlx

			//testing to see if it can place the line where it wants to
			if !p.checkVAlloc(desiredX) && !p.checkVAlloc(desiredX-1) && !p.checkVAlloc(desiredX+1) {
				//it can...
				p.setVAlloc(desiredX)
				//p.setVAlloc(desiredX - 1)
				//p.setVAlloc(desiredX + 1)
				p.drawVLine(desiredX, tly, tly+height-1, color)
				vLines = append(vLines, desiredX-tlx)
				break // exiting the line drawing attempt loop
			}
		}
	}

	if len(vLines) == 0 || len(hLines) == 0 {
		//invalid generation, must redo the entire pile
		return false
	}

	if isTarget {
		p.HorzLineCount = len(hLines)
		p.VertLineCount = len(vLines)

		//Testing for spacing
		minSpacing := 64
		for i := 0; len(vLines) > i; i++ {
			for j := 0; len(vLines) > j; j++ {
				if i == j {
					continue
				}

				if intAbs(vLines[i]-vLines[j]) < minSpacing {
					minSpacing = intAbs(vLines[i] - vLines[j])
				}
			}
		}

		if minSpacing == 2 {
			p.Spacing = p1SpacingVert
		} else {
			p.Spacing = p1SpacingNone
		}

		minSpacing = 64
		for i := 0; len(hLines) > i; i++ {
			for j := 0; len(hLines) > j; j++ {
				if i == j {
					continue
				}

				if intAbs(hLines[i]-hLines[j]) < minSpacing {
					minSpacing = intAbs(hLines[i] - hLines[j])
				}
			}
		}

		if minSpacing == 2 && p.Spacing == p1SpacingVert {
			p.Spacing = p1SpacingBoth
		} else if minSpacing == 2 {
			p.Spacing = p1SpacingHorz
		}

		//Testing for geometry
		//if it is an L, then one of the horizontal lines will be 0 and one of the vertical lines will be 0
		// -- OR --
		//one will be width - 1 and the other will be height -1
		// -- OR --
		//any combination of the above

		hasHExtreme := false
		hasVExtreme := false
		for i := 0; len(hLines) > i; i++ {
			if hLines[i] == 0 || hLines[i] == height-1 {
				hasHExtreme = true
			}
		}

		for i := 0; len(vLines) > i; i++ {
			if vLines[i] == 0 || vLines[i] == width-1 {
				hasVExtreme = true
			}
		}

		if hasHExtreme && hasVExtreme {
			p.Geometry = p1GeometryL
		} else if hasHExtreme || hasVExtreme {
			//If it is missing one of the extremes but has one, then it is a T
			p.Geometry = p1GeometryT
		} else {
			p.Geometry = p1GeometryNone
		}

		p.hLines = hLines
		p.vLines = vLines
		p.tlx = tlx
		p.tly = tly
	}

	return true
}

func (p *Project1Fa21) generatePile() bool {
	//must generate what colors are generated from bottom to top
	//each color is unique to a part, so once a color is put in a position, it cannot be used again

	colors := make([]int, 7)
	for i := 0; 7 > i; i++ {
		colors[i] = 0
	}

	for i := 0; 7 > i; i++ {
		for true {
			c := rand.Intn(7) + 1
			unique := true
			for j := 0; j < i; j++ {
				if colors[j] == c {
					unique = false
					break
				}
			}

			if unique {
				colors[i] = c
				break
			}
		}
	}

	//clearing the pile
	for i := 0; 1024 > i; i++ {
		p.Pile[i] = 0
	}

	p.horzAllocs = 0
	p.vertAllocs = 0

	//now to generate the parts
	for _, v := range colors {
		if !p.generatePart(v, v == int(p.TargetColor)) {
			//must redo the generation
			return false
		}
	}

	//will detect the obscurity type when it validates the solution
	return true
}

func (p *Project1Fa21) validatePile() bool {
	//performing the brute-force method for solving the problem
	//so, if you are a student trying to read this for hints on how to solve P1-2,
	//it won't get you a very good score...

	minX := 64
	minY := 64
	maxX := 0
	maxY := 0

	for y := 1; y < 63; y++ {
		for x := 1; x < 63; x++ {
			c := p.getPoint(x, y)
			if c == p.TargetColor {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	//Testing if it is still at least 25 x 25
	if maxX-minX+1 < 25 {
		return false // less than 25 px wide
	}
	if maxY-minY+1 < 25 {
		return false // less than 25 px tall
	}

	p.Solution = (uint32((minY*64)+minX) << 16) | uint32((maxY*64)+maxX)

	//Testing for obscurities
	horzObscured := false
	for _, v := range p.hLines {
		y := v + p.tly
		if p.getPoint(minX, y) != p.TargetColor {
			//obscured
			horzObscured = true
			break
		}
		if p.getPoint(maxX, y) != p.TargetColor {
			//obscured
			horzObscured = true
			break
		}
	}

	vertObscured := false
	for _, v := range p.vLines {
		x := v + p.tlx
		if p.getPoint(x, minY) != p.TargetColor {
			//obscured
			vertObscured = true
			break
		}
		if p.getPoint(x, maxY) != p.TargetColor {
			//obscured
			vertObscured = true
			break
		}
	}

	if horzObscured && vertObscured {
		p.Obscurity = p1ObscurityBoth
	} else if horzObscured {
		p.Obscurity = p1ObscurityHorz
	} else if vertObscured {
		p.Obscurity = p1ObscurityVert
	} else {
		p.Obscurity = p1ObscurityNone
	}

	return true
}

func (inst *instance) swi598() {
	//memory address in register $1
	if !inst.regInitialized(1) {
		inst.reportError(eSoftwareInterruptParameter, "register $1 uninitialized for swi 582 call. $1 should hold the Pile memory pointer")
	}

	p := new(Project1Fa21)
	p.ReportedAnswer = 0x12345678

	p.TargetColor = uint32(rand.Intn(7) + 1)
	inst.regWrite(3, p.TargetColor)

	//generating field
	for i := 0; true; i++ {
		if i > 100 {
			i = 0
			//Watchdog to prevent infinite field generation in extreme edge case
			fmt.Println("Randomization watchdog intervened")
			rand.Seed(time.Now().UnixNano())
		}

		if !p.generatePile() {
			//must try again, it failed to generate a valid field
			continue
		}

		//validating the pile, generating solution, and detecting obscurity type
		if !p.validatePile() {
			//must try again, invalid field generated with no solution within spec
			continue
		}

		//finished, generated valid pile
		break
	}

	memLoc := inst.regAccess(1)

	//storing pile in memory
	for i := 0; 1024 > i; i++ {
		inst.memWrite(memLoc+uint32(i)*4, p.Pile[i], 0xFFFFFFFF)
	}

	inst.swiContext = p
}

func (inst *instance) swi599() {
	//getting project info
	var p *Project1Fa21
	p, ok := inst.swiContext.(*Project1Fa21)
	if !ok {
		inst.reportError(eInvalidSoftwareInterrupt, "cannot use swi 599 with the previous swi call(s)")
		return
	}

	//offset in register $2
	if !inst.regInitialized(1) {
		inst.reportError(eSoftwareInterruptParameter, "register $2 uninitialized for swi 599 call. "+
			"$2 should hold the packed byte offsets of the top left and bottom right corners.")
	}

	p.ReportedAnswer = inst.regAccess(2)
	if (p.ReportedAnswer&0xFFFF) > 4096 || (p.ReportedAnswer>>16) > 4096 {
		inst.reportError(eSoftwareInterruptParameterValue, "%h is an invalid solution for swi 599. Reported "+
			"byte offsets must correspond to a pixel within the image, and the reported solution reports a number "+
			"too large to be on the image.", p.ReportedAnswer)
		return
	}

	//storing solution
	inst.regWrite(3, p.Solution)
}

func (v *VetSession) vetP1Fa21Interop(result EmulationResult) {
	v.TotalCount++

	p, ok := result.SWIContext.(*Project1Fa21)
	if !ok {
		//fatal error, software interrupts not called for the vet case
		fmt.Println("FATAL: Software interrupt swi 598 not called for the P1 vet, terminating emulation..")
		exit()
	}

	if p.ReportedAnswer == 0x12345678 {
		//no guess was made
		result.Errors = append(result.Errors, RuntimeError{
			EType:   eNoAnswerReported,
			Message: "No call was made to swi 599 ",
		})
	}
	if p.ReportedAnswer == p.Solution {
		//correct
		v.CorrectCount++
	}

	//create test case string
	obsStr := ""
	switch p.Obscurity {
	case p1ObscurityNone:
		obsStr = "ObsNone"
		break
	case p1ObscurityHorz:
		obsStr = "ObsHorz"
		break
	case p1ObscurityVert:
		obsStr = "ObsVert"
		break
	case p1ObscurityBoth:
		obsStr = "ObsBoth"
	}

	geoStr := ""
	switch p.Geometry {
	case p1GeometryNone:
		geoStr = "GeoNone"
		break
	case p1GeometryL:
		geoStr = "GeoL"
		break
	case p1GeometryT:
		geoStr = "GeoT"
		break
	}

	spaceStr := ""
	switch p.Spacing {
	case p1SpacingNone:
		spaceStr = "SpaceNone"
		break
	case p1SpacingHorz:
		spaceStr = "SpaceHorz"
		break
	case p1SpacingVert:
		spaceStr = "SpaceVert"
		break
	case p1SpacingBoth:
		spaceStr = "SpaceBoth"
	}

	tCase := "P1-" + obsStr + "-" + spaceStr + "-" + geoStr + "-" + strconv.Itoa(p.HorzLineCount) + "hLines-" +
		strconv.Itoa(p.VertLineCount) + "vLines"

	tcs, ok := v.TestCases[tCase]
	if ok {
		ef := tcs.ErrorsFrequency
		addVetErrors(result.Errors, ef)
		v.TestCases[tCase].TotalErrors = tcs.TotalErrors + len(result.Errors)
		v.TestCases[tCase].ErrorsFrequency = ef
		if p.ReportedAnswer == p.Solution {
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
		if p.ReportedAnswer == p.Solution {
			v.TestCases[tCase].Successes = 1
			v.TestCases[tCase].Fails = 0
		} else {
			v.TestCases[tCase].Successes = 0
			v.TestCases[tCase].Fails = 1
			v.addVetFailedSnap(result, tCase)
		}
	}
}

func drawBox(img *image.RGBA, x, y, width, height int, c color.Color) {
	for i := x; i < x+width; i++ {
		img.Set(i, y, c)
		img.Set(i, y+height-1, c)
	}

	for i := y; i < y+height; i++ {
		img.Set(x, i, c)
		img.Set(x+width-1, i, c)
	}
}

func genImageP1Fa21(res *EmulationResult) {
	var context *Project1Fa21
	context = res.SWIContext.(*Project1Fa21)

	scale := 4
	img := image.NewRGBA(image.Rect(0, 0, 64*scale, 64*scale))

	for y := 0; 64 > y; y++ {
		for x := 0; 64 > x; x++ {
			c := context.getPoint(x, y)

			var finalColor color.Color

			switch c {
			case 1:
				finalColor = color.RGBA{
					R: 254,
					G: 128,
					B: 129,
					A: 255,
				}
				break
			case 2:
				finalColor = color.RGBA{
					R: 219,
					G: 35,
					B: 0,
					A: 255,
				}
				break
			case 3:
				finalColor = color.RGBA{
					R: 0,
					G: 127,
					B: 0,
					A: 255,
				}
				break
			case 4:
				finalColor = color.RGBA{
					R: 2,
					G: 70,
					B: 255,
					A: 255,
				}
				break
			case 5:
				finalColor = color.RGBA{
					R: 255,
					G: 102,
					B: 52,
					A: 255,
				}
				break
			case 6:
				finalColor = color.RGBA{
					R: 229,
					G: 255,
					B: 0,
					A: 255,
				}
				break
			case 7:
				finalColor = color.RGBA{
					R: 153,
					G: 205,
					B: 255,
					A: 255,
				}
				break
			default:
				finalColor = color.RGBA{
					R: 0,
					G: 0,
					B: 0,
					A: 255,
				}
			}

			for i := 0; scale > i; i++ {
				for j := 0; scale > j; j++ {
					img.Set(x*scale+j, y*scale+i, finalColor)
				}
			}
		}
	}

	//drawing code-submitted box
	if context.ReportedAnswer != 0x12345678 {
		//the code did submit a bounding box, so will draw it

		tlx := int((context.ReportedAnswer >> 16) % 64)
		tly := int((context.ReportedAnswer >> 16) / 64)
		brx := int((context.ReportedAnswer & 0xFFFF) % 64)
		bry := int((context.ReportedAnswer & 0xFFFF) / 64)

		drawBox(img, tlx*scale, tly*scale, scale, scale, color.White)
		drawBox(img, brx*scale, bry*scale, scale, scale, color.White)
		drawBox(img, tlx*scale, tly*scale, (brx-tlx+1)*scale, (bry-tly+1)*scale, color.White)
	}

	//drawing the solution box
	tlx := int((context.Solution >> 16) % 64)
	tly := int((context.Solution >> 16) / 64)
	brx := int((context.Solution & 0xFFFF) % 64)
	bry := int((context.Solution & 0xFFFF) / 64)

	solGreen := color.RGBA{
		R: 0,
		G: 255,
		B: 60,
		A: 255,
	}

	drawBox(img, tlx*scale, tly*scale, scale, scale, solGreen)
	drawBox(img, brx*scale, bry*scale, scale, scale, solGreen)
	drawBox(img, tlx*scale, tly*scale, (brx-tlx+1)*scale, (bry-tly+1)*scale, solGreen)

	f, err := os.Create("testCase.png")
	if err != nil {
		fmt.Println("Failed to create test case image: " + err.Error())
		return
	}
	defer f.Close()
	err = png.Encode(f, img)
	if err != nil {
		fmt.Println("Failed to encode test case image: " + err.Error())
		return
	}

	fmt.Println("Saved image of test case. Name: testCase.png")
}

func genFa21Project1Dump(res *EmulationResult) {
	var context *Project1Fa21
	context = res.SWIContext.(*Project1Fa21)

	builder := strings.Builder{}
	for i, v := range context.Pile {
		builder.WriteString(strconv.Itoa(5356 + 4*i))
		builder.WriteByte(':')
		temp := strconv.FormatUint(uint64(v), 10)
		builder.WriteString(strings.Repeat(" ", 11-len(temp)))
		builder.WriteString(temp)
		builder.WriteByte('\n')
	}

	fName := "pile_" + strconv.Itoa(int(context.Solution>>16)) + "_" +
		strconv.Itoa(int(context.Solution&0xFFFF)) + "_" + strconv.Itoa(int(context.TargetColor)) + ".txt"
	e := ioutil.WriteFile(fName, []byte(builder.String()), 0644)
	if e != nil {
		fmt.Println("ERROR: Failed to save dump file:", e.Error())
		return
	}

	fmt.Println("Saved dump file. Name: " + fName)
}
