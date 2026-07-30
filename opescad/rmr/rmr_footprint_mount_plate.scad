
// RMR footprint mount plate - OpenSCAD model
// Units: mm

$fs=200;
$fn=50;


plate_length = 45;
plate_width = 30;
plate_thickness = 4;
screw_hole_diameter = 3.2;
pin_hole_diameter = 2.0;

screw_spacing_x = 18.8;
screw_offset_y = 7.0; // odległość od środka płyty
pin_spacing_x = 22.5;

pin_to_screws=23.323;
pin_offset_y = screw_offset_y - pin_to_screws;

module rmr_plate() {
    difference() {
        // Base plate
        cube([plate_width,plate_length, plate_thickness], center=true);

        // Screw holes (M3 or #6-32)
        translate([-screw_spacing_x/2, screw_offset_y, 0])
            cylinder(h=10, d=screw_hole_diameter, center=true);
        
        translate([ screw_spacing_x/2, screw_offset_y, 0])
            cylinder(h=10, d=screw_hole_diameter, center=true);

  
    }
          // Pin holes (optional)
        translate([-pin_spacing_x/2, pin_offset_y, 0])
            cylinder(h=10, d=pin_hole_diameter, center=true);
        translate([ pin_spacing_x/2, pin_offset_y, 0])
            cylinder(h=10, d=pin_hole_diameter, center=true);
}

rmr_plate();
