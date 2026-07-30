package main

import (
	"image/color"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type position struct {
	x, y float64
}

type velocity struct {
	x, y float64
}

type size struct {
	x, y float64
}

type gravity struct {
}

type entity struct {
	position *position
	velocity *velocity
	size     *size
	gravity  *gravity
}

type system interface {
	update([]entity) []entity
}

type ecs struct {
	entities []entity
	systems  []system
}

func (e *ecs) update() {
	for _, s := range e.systems {
		e.entities = s.update(e.entities)
	}
}

type velocitySystem struct{}

func (s *velocitySystem) update(entities []entity) []entity {
	for _, e := range entities {

		if e.position != nil && e.velocity != nil {
			e.position.x += e.velocity.x
			e.position.y += e.velocity.y
		}

	}
	return entities
}

type gravitySystem struct{}

func (s *gravitySystem) update(entities []entity) []entity {
	for _, e := range entities {
		if e.gravity != nil && e.velocity != nil {
			e.velocity.y = e.velocity.y + .05
		}
	}
	return entities
}

type leftRightBounce struct{}

func (s *leftRightBounce) update(entities []entity) []entity {
	for _, e := range entities {
		if e.position != nil && e.size != nil && e.velocity != nil {
			if e.position.x <= 0 || e.size.y+e.position.x >= 800 {
				e.velocity.x = -e.velocity.x
			}
		}
	}

	return entities
}

type Game struct {
	count int64
	posY  float64
	speed float64

	rectX, rectY          float64
	rectWidth, rectHeight float64
	velocityX, velocityY  float64
}

func (g *Game) Update() error {
	g.count++

	g.posY *= g.speed
	g.speed = g.speed * .7

	g.rectX += g.velocityX
	g.rectY += g.velocityY

	g.velocityY = g.velocityY + .05

	// Check for collision with window edges and reverse direction if necessary
	if g.rectX <= 0 || g.rectX+g.rectWidth >= 800 {
		g.velocityX = -g.velocityX
	}

	if g.rectY <= 0 {
		g.rectY = 0
		g.velocityY = math.Abs(g.velocityY) + 5
		g.rectHeight = math.Abs(g.rectHeight - math.Abs(g.velocityY)*5)
	} else if g.rectY+g.rectHeight >= 600 {
		g.rectHeight = g.rectHeight * 1.1
		g.velocityY = -float64(rand.Intn(10)) - 1
		g.rectY = 600 - g.rectHeight - 1
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {

	clr := color.RGBA{R: 255, G: 0, B: 0, A: 255} // Red color

	// Draw the rectangle
	ebitenutil.DrawRect(screen, g.rectX, g.rectY, g.rectWidth, g.rectHeight, clr)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return outsideWidth, outsideHeight
}

func main() {
	game := &Game{
		rectX:      100,
		rectY:      100,
		rectWidth:  200,
		rectHeight: 150,
		velocityX:  2,
		velocityY:  2,
	}
	ebiten.SetWindowSize(800, 600)
	ebiten.SetWindowTitle("Bouncing Rectangle")

	err := ebiten.RunGame(game)
	if err != nil {
		panic(err)
	}
}
