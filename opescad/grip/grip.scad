$fn = 64;

module ar15_grip_mount() {
    difference() {
        // Mounting block
        cube([30, 45, 20], center = false);

        // Screw hole
        translate([15, 22.5, -5])
            cylinder(h = 30, d = 6.3, center = false);

        // Anti-rotation slot
        translate([12, 0, 17])
            cube([6, 3, 10]);
    }
}

module simple_grip_body() {
    grip_width = 30;
    grip_depth = 25;
    grip_height = 95;
    grip_angle = -12;

    // Grip block, slightly slanted
    translate([0, 45, 0])  // attach to mount block bottom
    rotate([grip_angle, 0, 0])
        cube([grip_width, grip_depth, grip_height], center = false);
}

// Union grip with mount block
union() {
    ar15_grip_mount();
    simple_grip_body();
}