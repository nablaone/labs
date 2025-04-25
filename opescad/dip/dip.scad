$fn=100;

// Volume ccm
V=3.8; //[1:20]


// prints 
Ve1=380; // expected
Va1=9.8*3.14*((6.8/2)^2); // printed
shrinkF1=Va1/Ve1; // shrink factor
echo("SHRINK1",Va1,Ve1,shrinkF1); 


Ve2=350;
Va2=8.85*3.14*((6.75/2)^2);
shrinkF2=Va2/Ve2;
echo("SHRINK2",Va2,Ve2,shrinkF2); 

// 
shrinkF=(shrinkF1+shrinkF2)/2;

echo("Shrink", shrinkF);




r=3.5;

h=V*(1/shrinkF)*100/(3.14*r*r);
echo(r,h);

wall=1;

echo("Computed VOLUME", h * 3.14  * r *r);

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

