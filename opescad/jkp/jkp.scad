
$fn=200;

size=80;

font="Solid Stencil 2023";
text="JKP";
xpadding=10;

padding=19;

font_w= size*2;
font_h = size;

width=font_w+2*padding;
height=font_h+2*padding;

echo(width,height);
thickness=1;



difference() {
cube([width, height,thickness]);

translate([xpadding + width/2-font_w/2, height/2 - font_h/2   ,- thickness])
linear_extrude(height = 11)
    text(text, font=font, size=size);
}