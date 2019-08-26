function ECS() {
    this.entityCounter = 0;

    this.events = {};
    this.entities= [];
    this.systems = [];
    this.components = {};
    this.frame = 0;
}


ECS.prototype = {
   
    newEntity: function (name) {
        let entity = {
            name: name
        };
        this.entities.push(entity);

        return entity;
    },

    removeEntity: function(entity) {

        for(var i in entity) {
            if (this.components[i] != undefined) {
               this.removeComponent(entity,i);
            }
        }

        var idx  = this.entities.indexOf(entity);
        this.entities.splice(idx,1);

    },

    registerEvent: function(name) {
        this.events[name] = [];
    },

    addEventListener: function(name, fn) {
        this.events[name].push(fn);
    },

    trigger: function(name, arg) {
        var fns = this.events[name];

        for(var i = 0; i < fns.length; i++) {
            fns[i](arg);
        }
    },



    addComponent: function (entity, name) {

        var c = this.components[name];

        if (c == undefined) {
            console.log("No component", name)
            return null;
        }

        var component = c.add();
        entity[name] = component;

        return component;

    },

    removeComponent: function(entity, name) {

        var component = this.getComponent(entity, name);

        this.components[name].remove(component);
        entity[component]= null;
    },

    getComponent: function (entity, component) {

        if (entity[component] != undefined && entity[component] != null) {
            return entity[component];
        }
        return null;
    },

    hasComponent: function(entity, component) {

        if (this.getComponent(entity,component) != null) {
            return true;
        }
        return false;
    },

    filter: function (component) {
        var res = [];

        for (var i = 0; i < this.entities.length; i++) {
            if (this.hasComponent(this.entities[i], component)) {
                res.push(this.entities[i]);
            }
        }

        return res;
    },


    addSystem: function(system) {
        this.systems.push(system);
    },

    cycle: function () {
        this.frame++;
        for (var i = 0; i < this.systems.length; i++) {
            var system = this.systems[i];
            //console.log("updating system: ", system)
            system(ecs);
        }
    }

};

function VelocitySystem(ecs) {

    var entities = ecs.filter("velocity");

    for (let i = 0; i < entities.length; i++ ) {

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

    for (let i = 0; i < entities.length; i++ ) {

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

    for (let i = 0; i < entities.length; i++ ) {

        var e = entities[i];     
        var pos = ecs.getComponent(e, "position");
        
        console.log("POSITION: ", e, pos.x, pos.y);
    }

};





function BoundarySystem(esc) {

    var entities = ecs.filter("bouncingRoof");

    for (let i = 0; i < entities.length; i++ ) {

        var e = entities[i];     
        var pos = ecs.getComponent(e, "position");
        var velocity = ecs.getComponent(e, "velocity");
        
        if (pos.y > HEIGHT) {
            pos.y = HEIGHT;
            velocity.y = - velocity.y;
        }
    }
    

    var entities = ecs.filter("bouncingFloor");

    for (let i = 0; i < entities.length; i++ ) {

        var e = entities[i];     
        var pos = ecs.getComponent(e, "position");
        var velocity = ecs.getComponent(e, "velocity");
        
        if (pos.y < 0) {
            pos.y = 0;
            velocity.y = - velocity.y;
        }
    }


    var entities = ecs.filter("dampingFloor");

    for (let i = 0; i < entities.length; i++ ) {

        var e = entities[i];     
        var pos = ecs.getComponent(e, "position");
        var velocity = ecs.getComponent(e, "velocity");
        
        if (pos.y < 0) {
            pos.y = 0;
            velocity.y = 0;
        }
    }

    var entities = ecs.filter("boundaryBounce");

    for (let i = 0; i < entities.length; i++ ) {

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

