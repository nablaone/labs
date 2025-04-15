$fn=100;
// Volume
V=3.5; //[1:20]


r=3.5;

h=V*100/(3.14*r*r);
echo(r,h);

wall=1;

echo("VOLUMNE", h * 3.14  * r *r);

handle=90;
handleZ=3;

difference() {
union() {

color("green") 
difference() {
hull() {
  cylinder(handleZ, r/2,r/2);
    
  translate([handle,0,0])
    cylinder(handleZ, r+wall, r+wall);     
}
 translate([40,-2,handleZ-0.5]) 
  linear_extrude(height = handleZ )
    text(str(V,"cc"), font="Futura", size=4);
}
color("red")
cylinder(h+wall, r+wall, r+wall);

}

// cut the cylinder, 
// don't care what was before
union() {
translate([0,0,wall])
 cylinder(h+5, r,r);
translate([0,0,wall+h]) 
  cylinder(h, 2*r, 2*r);
}
}

