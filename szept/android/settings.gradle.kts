rootProject.name = "szept"

pluginManagement {
    repositories {
        google()
        gradlePluginPortal()
        mavenCentral()
    }
}

dependencyResolutionManagement {
    repositories {
        google()
        mavenCentral()
    }
}

include(":crypto") // protocol crypto; plain JVM
include(":core")   // client logic: state, server API, engine; plain JVM
include(":app")    // Android UI
