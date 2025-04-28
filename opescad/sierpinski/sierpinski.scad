// Sierpinski carpet
m=-0.5;
M=2.5;

module sierpinski(x,y,w,level) {
    
    if (level > 0) {
      
    w3=w/3;
    w23=2*w/3;
        
    level = level -1;
    
    union() {
        
        translate([x+w3,y+w3,m])
          cube([w3,w3,M]);
        
        translate([m,x+w3,y+w3])
          cube([M,w3,w3]);
     
        translate([x+w3,m,y+w3])
          cube([w3,M,w3]);
        
        sierpinski(x,y     ,    w3,level);
        sierpinski(x,y + w3, w3,level);
        sierpinski(x,y + w23, w3,level);

        sierpinski(x+w23,y , w3, level); 
        sierpinski(x+w23,y+w3, w3, level);
        sierpinski(x+w23,y+w23,w3,level);
        
        sierpinski(x+ w3, y, w3,level);
        sierpinski(x + w3, y + w23, w3 ,level);
    }

   };
    
}


difference() {
  cube([1,1,1]);
  // cut out the non-sierpinski part
  sierpinski(0,0,1,3);
}