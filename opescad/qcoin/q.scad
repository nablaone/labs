
$fn=100;

R=31.4/2;

D=3;

d=1;

rim=2;


module coin() {

color("green")
difference() {
    
cylinder(D,R,R);
color("yellow")
translate([0,0,D-d]) cylinder(d, R-rim, R-rim);
}

color("red")
translate([0,0,D-d])
linear_extrude(height = d)
    translate([-15.5, -14])  // Position it
    scale([0.11, 0.11])      // Shrink if it's too big
        import("q.svg"); 


}


ridge_count = 120;
ridge_depth = 0.3;
ridge_width = 0.4;

module reeded_edge() {
    for (i = [0 : 360/ridge_count : 360 - 360/ridge_count]) {
        rotate([0,0,i])
        translate([R - ridge_depth/2, -ridge_width/2, 0])
        cube([ridge_depth, ridge_width, D]);
    }
}


difference() {
coin();
reeded_edge();    
}
