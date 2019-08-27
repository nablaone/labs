
console.log("game.js");

WIDTH= 700;
HEIGHT= 400;

function initKonva() {
    // first we need to create a stage

    var stage = new Konva.Stage({
        container: 'container',   // id of container <div>
        width: WIDTH,
        height: HEIGHT
    });

    // then create layer
    var layer = new Konva.Layer();


    // add the layer to the stage
    stage.add(layer);

    var container = stage.container();

      // make it focusable

    container.tabIndex = 1;
      // focus it
      // also stage will be in focus on its click
    container.focus();


    return layer;
};

function VelocitySystem(ecs) {

    var entities = ecs.filter("velocity");

    for (var i = 0; i < entities.length; i++ ) {

        var e = entities[i];     

        if (!ecs.hasComponent(e,"position")) {
            continue;
        }

        var pos = ecs.getComponent(e, "position");
        var velocity = ecs.getComponent(e, "velocity");


        pos.x = pos.x + velocity.x;
        pos.y = pos.y + velocity.y;

    }

};

function GravitySystem(ecs) {

    var entities = ecs.filter("gravity");

    for (var i = 0; i < entities.length; i++ ) {

        var e = entities[i];     

        if (!ecs.hasComponent(e,"velocity")) {
            continue;
        }



        var velocity = ecs.getComponent(e, "velocity");

        velocity.y = velocity.y - 0.1;
        
  }

};


function DumpPositionSystem(ecs) {

    var entities = ecs.filter("position");

    for (var i = 0; i < entities.length; i++ ) {

        var e = entities[i];     
        var pos = ecs.getComponent(e, "position");
        
        console.log("POSITION: ", e, pos.x, pos.y);
    }

};





function BoundarySystem(esc) {

    var entities = ecs.filter("bouncingRoof");

    for (var i = 0; i < entities.length; i++ ) {

        var e = entities[i];     
        var pos = ecs.getComponent(e, "position");
        var velocity = ecs.getComponent(e, "velocity");
        
        if (pos.y > HEIGHT) {
            pos.y = HEIGHT;
            velocity.y = - velocity.y;
        }
    }
    

    var entities = ecs.filter("bouncingFloor");

    for (var i = 0; i < entities.length; i++ ) {

        var e = entities[i];     
        var pos = ecs.getComponent(e, "position");
        var velocity = ecs.getComponent(e, "velocity");
        
        if (pos.y < 0) {
            pos.y = 0;
            velocity.y = - velocity.y;
        }
    }


    var entities = ecs.filter("dampingFloor");

    for (var i = 0; i < entities.length; i++ ) {

        var e = entities[i];     
        var pos = ecs.getComponent(e, "position");
        var velocity = ecs.getComponent(e, "velocity");
        
        if (pos.y < 0) {
            pos.y = 0;
            velocity.y = 0;
        }
    }

    var entities = ecs.filter("boundaryBounce");

    for (var i = 0; i < entities.length; i++ ) {

        var e = entities[i];     
        var pos = ecs.getComponent(e, "position");
        var velocity = ecs.getComponent(e, "velocity");
        
        if (pos.x < 0) {
            pos.x = 0;
            velocity.x = - velocity.x;
        }

        if (pos.x > WIDTH) {
            pos.x = WIDTH;
            velocity.x = - velocity.x;
        }

        if (pos.y < 0) {
            pos.y = 0;
            velocity.y = - velocity.y;
        }

        if (pos.y > HEIGHT) {
            pos.y = HEIGHT;
            velocity.y = - velocity.y;
        }

    }
}

function CollisionDetectorSystem(ecs) {

    var players = ecs.filter("player");
    var collidables = ecs.filter("collidable");

    for (var i = 0; i < players.length; i++ ) {

        var p = players[i];     

        var ppos = ecs.getComponent(p,'position');
        var pplayer = ecs.getComponent(p,'player');

        if (ppos == null || pplayer == null) {
            continue;
        }

        var pmaxx = ppos.x + pplayer.width;
        var pmaxy = ppos.y + pplayer.height;
        
        var inside = function(x,y) {
            var res =  x >= ppos.x && x <= pmaxx && y >= ppos.y && y <= pmaxy;

            //console.log("INSIDE:", ppos.x, ppos.y, pmaxx, pmaxy, x,y);
            return res;
        }
        
        var collision = function(c) {
   
            var pos = ecs.getComponent(c,'position');
            var col = ecs.getComponent(c,'collidable');

            if (pos == null || col == null) {
                return false;
            }

            var cmaxx = pos.x + col.width;
            var cmaxy = pos.y + col.height;

            return inside(pos.x, pos.y) || inside(pos.x, cmaxy) || inside(cmaxx, pos.y) || inside(cmaxx, cmaxy);

        }


        for(var j = 0; j < collidables.length; j++) {

            var c = collidables[j];

            if (collision(c)) {
        
                ecs.trigger("collision",c);
            }
        }

    }

}
 

function KonvaPositionUpdateSystem(ecs) {

    var entities = ecs.filter("drawable");

    for (var i = 0; i < entities.length; i++ ) {

        var e = entities[i];     

        if (!ecs.hasComponent(e,"position")) {
            continue;
        }

        var drawable = ecs.getComponent(e, "drawable");
        var position = ecs.getComponent(e, "position");
    
        drawable.element.setX(position.x);
        drawable.element.setY(HEIGHT - position.y - drawable.element.getHeight());
    }
};




function CleanUpSpritesSystem(ecs) {

    var entities = ecs.filter("drawable");

    for (var i = 0; i < entities.length; i++ ) {

        var e = entities[i];     

        if (!ecs.hasComponent(e,"position")) {
            continue;
        }

        var drawable = ecs.getComponent(e, "drawable");
        var position = ecs.getComponent(e, "position");

        if (position.x < 0 || position.y < 0) {
            ecs.removeEntity(e);
        }
    }
}


function initECS(layer) {

    var ecs = new ECS();

    window.ecs = ecs;

    ecs.components.position = {
            add: function () {

                return {
                    x: 0,
                    y: 0,
                };
            },

            remove: function () {

            },


        };

    ecs.components.velocity = {
  
            add: function () {

                return {
                    x: 0,
                    y: 0,
                };
            },

            remove: function () {

            },
        };

    var empty = function() {
        return {
            add: function(){return {}},
            remove: function(){}
        }
    };    



    ecs.components.boundaryBounce = empty();
    ecs.components.gravity = empty();
    ecs.components.dampingFloor = empty();
    ecs.components.bouncingRoof = empty();
    ecs.components.bouncingFloor = empty();
    ecs.components.player = empty();

    ecs.components.collidable = {
        add: function(){return {
            kill: false,
            points: 0,
            width: 0,
            height: 0,
        }},
        remove: function(){}
    }; 

    ecs.components.drawable = {

        add: function() {
            return {element: null};
        },

        remove: function(instance) {
            instance.element.remove();
        },
    }
    
    function MapGenerator(ecs) {

        if(ecs.frame % 200 != 0) {
            return;
        }

        e1 = ecs.newEntity("e" + ecs.frame);

        var collidable = ecs.addComponent(e1, "collidable");
        collidable.width = 30;
        collidable.height = 30;
        ecs.addComponent(e1, "bouncingFloor");
        ecs.addComponent(e1, "bouncingRoof");

        var pos = ecs.addComponent(e1,"position");
        pos.x = WIDTH;
        pos.y = HEIGHT * Math.random();

        var vel = ecs.addComponent(e1,"velocity");

        vel.x = -1;
        vel.y = 0;

        var drawable = ecs.addComponent(e1,"drawable");

  //     create our shape
        var el = new Konva.Rect({
            x: 0,
            y: 0,
            width: collidable.width,
            height: collidable.height,
            fill: 'red',
            stroke: 'black',
            strokeWidth: 4,
        });
        layer.add(el)

        drawable.element = el;

        // kupa czy lizak
        if (Math.random() < 0.5) {
            el.setFill("brown");
            collidable.points = -1;
            pos.y = 0;
        } else {
            el.setFill("#ff0000");
            collidable.points = 1;
        }
    };

    var KonvaRedraw = function() {
        layer.draw();
    };


    ecs.registerEvent("jump");

    ecs.addEventListener("jump", function(arg) {
        console.log("JUMP"); 
    });

    ecs.addEventListener("jump", function(arg) {
        var entities = ecs.filter("player");

        for (var i = 0; i < entities.length; i++ ) {

            var e = entities[i];     

            if (!ecs.hasComponent(e,"position")) {
                continue;
            }

            if (!ecs.hasComponent(e,"velocity")) {
                continue;
            }

            var velocity = ecs.getComponent(e, "velocity");
            var position = ecs.getComponent(e, "position");
        
            if (position.y > 0) {
                continue;
            }
            velocity.y = 8;
        }
    });

    window.addEventListener("keydown", function(e) {

        if (e.keyCode == 32) {
            ecs.trigger("jump", null);
            e.preventDefault();
            return false;
        }
        return;
    })

    // add touch listener

    ecs.registerEvent("collision");
    ecs.addEventListener("collision", function(entity) {

        console.log("COLISION", entity);
        var col = ecs.getComponent(entity, "collidable");

        if (col.points > 0) {

            console.log("POINTS");
            var vel = ecs.getComponent(entity, "velocity");
            vel.x = 1;
            vel.y = 3;
            ecs.addComponent(entity, "gravity");
        
            var drawable = ecs.getComponent(entity, "drawable");
            drawable.element.rotate(45);
            drawable.element.setFill("white");

        } else {
            console.log("SHIT");

            var drawable = ecs.getComponent(entity, "drawable");

            drawable.element.setHeight(10);
            drawable.element.setWidth(50);

        }

        ecs.removeComponent(entity, "collidable");
        ecs.removeComponent(entity, "bouncingFloor");
        ecs.removeComponent(entity, "dampingFloor");
    });

    ecs.addSystem(CollisionDetectorSystem);
    ecs.addSystem(MapGenerator);
    ecs.addSystem(VelocitySystem);
    ecs.addSystem(GravitySystem);
   // ecs.addSystem(DumpPositionSystem);
    ecs.addSystem(BoundarySystem);
    ecs.addSystem(KonvaPositionUpdateSystem);
    ecs.addSystem(KonvaRedraw);
    ecs.addSystem(CleanUpSpritesSystem);

    setInterval(ecs.cycle.bind(ecs), 20);

   
    window.ecs = ecs;
    return ecs;
}

function createPlayer(layer,ecs) {
    
    player = ecs.newEntity("player");
    ecs.addComponent(player,"dampingFloor")
    var vel = ecs.addComponent(player,"velocity");
    vel.x = 0;
    vel.y = 0;
    var pos = ecs.addComponent(player,"position");
    pos.x=100;
    pos.y=250;
    ecs.addComponent(player, "gravity");
    var drawable = ecs.addComponent(player, "drawable");

    var pc = ecs.addComponent(player, "player");
    pc.width = 100;
    pc.height = 100;

    var el = new Konva.Rect({
        x: 0,
        y: 0,
        width: pc.width,
        height: pc.height,
        stroke: 'black',
        strokeWidth: 1,
    });
    layer.add(el);

    drawable.element = el;
   
}

function init() {
    var layer = initKonva();

    var ecs = initECS(layer);

    createPlayer(layer,ecs);
    
}