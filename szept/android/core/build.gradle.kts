plugins {
    kotlin("jvm")
}

java {
    sourceCompatibility = JavaVersion.VERSION_17
    targetCompatibility = JavaVersion.VERSION_17
}

kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
    }
}

dependencies {
    api(project(":crypto"))
    api("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.10.2")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    // Android ships org.json in the platform; plain JVM tests add it.
    compileOnly("org.json:json:20240303")
    testImplementation("org.json:json:20240303")
    testImplementation(kotlin("test"))
}

tasks.test {
    useJUnitPlatform()
    // The integration test builds and runs the Go server from this repo.
    systemProperty("szeptRepo", rootProject.file("..").absolutePath)
}

dependencies {
    testImplementation("com.google.zxing:javase:3.5.3")
    testImplementation("com.squareup.okhttp3:okhttp:4.12.0")
}

// Dev tool: a scripted peer for testing the app on an emulator. See PeerBot.kt.
tasks.register<JavaExec>("peerBot") {
    classpath = sourceSets["test"].runtimeClasspath
    mainClass.set("szept.client.PeerBotKt")
    args = (project.findProperty("botArgs") as String?)?.split(" ") ?: emptyList()
}
