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
        var entity = {
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
        entity[name] = null;
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



