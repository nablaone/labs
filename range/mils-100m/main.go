package main

import (
	"fmt"
	"os"
)

const (
	pageWidthMM         = 210
	pageHeightMM        = 297
	boxSizeMM           = 200
	gridStepMM          = 10
	lineWidthMM         = 0.25
	circleRadiusMM      = 15
	textFontSize        = 4
	circleStrokeWidthMM = 10
)

func main() {
	file, err := os.Create("a4_centered_grid.svg")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	boxHalf := float64(boxSizeMM) / 2
	centerX := float64(pageWidthMM) / 2
	centerY := float64(pageHeightMM) / 2

	fmt.Fprintf(file, `<svg xmlns="http://www.w3.org/2000/svg" width="%dmm" height="%dmm" viewBox="0 0 %d %d">`,
		pageWidthMM, pageHeightMM, pageWidthMM, pageHeightMM)

	// Define a mask to cut out grid inside the circle
	fmt.Fprintf(file, `<defs>
  <mask id="gridMask">
    <rect x="0" y="0" width="100%%" height="100%%" fill="white"/>
    <circle cx="%f" cy="%f" r="%d" fill="black"/>
  </mask>
</defs>`, centerX, centerY, circleRadiusMM)

	// Draw the outer 200x200 mm box
	fmt.Fprintf(file, `<rect x="%f" y="%f" width="%d" height="%d" fill="none" stroke="#aaa" stroke-width="%f"/>`,
		centerX-boxHalf, centerY-boxHalf, boxSizeMM, boxSizeMM, lineWidthMM)

	// Draw grid with mask
	//fmt.Fprintln(file, `<g>`)
	for x := -boxSizeMM / 2; x <= boxSizeMM/2; x += gridStepMM {
		xPos := centerX + float64(x)
		fmt.Fprintf(file, `<line x1="%f" y1="%f" x2="%f" y2="%f" stroke="#aaa" stroke-width="%f"/>`,
			xPos, centerY-boxHalf, xPos, centerY+boxHalf, lineWidthMM)
	}
	for y := -boxSizeMM / 2; y <= boxSizeMM/2; y += gridStepMM {
		yPos := centerY + float64(y)
		fmt.Fprintf(file, `<line x1="%f" y1="%f" x2="%f" y2="%f" stroke="#aaa" stroke-width="%f"/>`,
			centerX-boxHalf, yPos, centerX+boxHalf, yPos, lineWidthMM)
	}
	//fmt.Fprintln(file, `</g>`)

	// Draw the circle

	drawCircle := func(offsetX, offsetY float64) {
		fmt.Fprintf(file, `<circle cx="%f" cy="%f" r="%d" fill="none" stroke="black" stroke-width="%d"/>`,
			centerX+offsetX, centerY+offsetY, circleRadiusMM, circleStrokeWidthMM)
	}

	qwadrantBox := 60.0

	drawCircle(0, 0)
	drawCircle(-qwadrantBox, -qwadrantBox)
	drawCircle(-qwadrantBox, qwadrantBox)
	drawCircle(qwadrantBox, -qwadrantBox)
	drawCircle(qwadrantBox, qwadrantBox)

	/*
		// Coordinate labels
		for x := -boxSizeMM / 2; x <= boxSizeMM/2; x += gridStepMM {
			if x == 0 {
				continue
			}
			xPos := centerX + float64(x)
			fmt.Fprintf(file, `<text x="%f" y="%f" font-size="%d" text-anchor="middle">%d</text>`,
				xPos+5, centerY-2, textFontSize, x/10)
		}
		for y := -boxSizeMM / 2; y <= boxSizeMM/2; y += gridStepMM {
			if y == 0 {
				continue
			}
			yPos := centerY + float64(y)
			fmt.Fprintf(file, `<text x="%f" y="%f" font-size="%d" stroke="gray" text-anchor="end" dominant-baseline="middle">%d</text>`,
				centerX+2, yPos-2, textFontSize, -y/10)
		}
	*/
	fmt.Fprintln(file, `</svg>`)
}
