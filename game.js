
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

    var empty = {
        add: function(){return {}},
        remove: function(){}
    };    


    ecs.components.boundaryBounce = {
        add: function(){return {}},
        remove: function(){}
    };    

    ecs.components.gravity = {
        add: function(){return {}},
        remove: function(){}
    };   

    ecs.components.dampingFloor = {
        add: function(){return {}},
        remove: function(){}
    }; 

    ecs.components.bouncingRoof = {
        add: function(){return {}},
        remove: function(){}
    }; 


    ecs.components.bouncingFloor = {
        add: function(){return {}},
        remove: function(){}
    }; 

    ecs.components.player = {
        add: function(){return {}},
        remove: function(){}
    }; 


    ecs.components.collidable = {
        add: function(){return {}},
        remove: function(){}
    }; 

    ecs.components.drawable = {

        add: function() {
            return {element: null};
        },

        remove: function(instance) {
            instance.remove();
        },
    }
    
    var KonvaPositionUpdateSystem = function(ecs) {

        var entities = ecs.filter("drawable");

        for (let i = 0; i < entities.length; i++ ) {

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

    var KonvaRedraw = function() {
        layer.draw();
    }

    var MapGenerator = function(ecs) {

        if(ecs.frame % 200 != 0) {
            return;
        }

        e1 = ecs.newEntity("e" + ecs.frame);
    
        ecs.addComponent(e1, "collidable");
        ecs.addComponent(e1, "bouncingFloor");
        ecs.addComponent(e1, "bouncingRoof");
        
        var pos = ecs.addComponent(e1,"position");
        pos.x = WIDTH;
        pos.y = HEIGHT * Math.random();

        var vel = ecs.addComponent(e1,"velocity");

        vel.x = -1;
        vel.y = 0;

        var drawable = ecs.addComponent(e1,"drawable");

      // create our shape
        var el = new Konva.Rect({
            x: 0,
            y: 0,
            width: 30,
            height: 30,
            fill: 'red',
            stroke: 'black',
            strokeWidth: 4,
        });
        layer.add(el)
        
        drawable.element = el;
        
        // kupa czy lizak
        if (Math.random() < 0.5) {
            el.setFill("brown");
            pos.y = 0;
        } else {
            el.setFill("#ff0000");
        }
    }

    function CleanUpSpritesSystem(ecs) {

        var entities = ecs.filter("drawable");

        for (let i = 0; i < entities.length; i++ ) {

            var e = entities[i];     

            if (!ecs.hasComponent(e,"position")) {
                continue;
            }

            var drawable = ecs.getComponent(e, "drawable");
            var position = ecs.getComponent(e, "position");

            if (position.x < 0) {
                ecs.removeEntity(e);
            }
        }
    }


    ecs.registerEvent("jump");

    ecs.addEventListener("jump", function(arg) {
        console.log("JUMP"); 
    });

    ecs.addEventListener("jump", function(arg) {
        var entities = ecs.filter("player");

        for (let i = 0; i < entities.length; i++ ) {

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

    var el = new Konva.Rect({
        x: 0,
        y: 0,
        width: 30,
        height: 30,
        fill: 'red',
        stroke: 'black',
        strokeWidth: 4,
    });
    layer.add(el);

    el.setFill("#000000");
    el.setWidth(100);
    el.setHeight(100);
    drawable.element = el;
    
    ecs.addComponent(player, "player");

}

function init() {
    var layer = initKonva();

    var ecs = initECS(layer);

    createPlayer(layer,ecs);
    
}