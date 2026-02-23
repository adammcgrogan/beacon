plugins {
    `java-library`
    `maven-publish`
    alias(libs.plugins.shadow)
    alias(libs.plugins.runpaper)
}

repositories {
    mavenLocal()
    mavenCentral()
    maven("https://repo.papermc.io/repository/maven-public/")
    maven("https://repo.maven.apache.org/maven2/")
}

dependencies {
    compileOnly(libs.paper)
    api(libs.websocket)
    compileOnly(libs.log4j)
}

group = "com.beacon"
version = "1.0.0"
description = "BeaconPlugin"
java.sourceCompatibility = JavaVersion.VERSION_21

tasks.withType<JavaCompile> {
    options.encoding = "UTF-8"
}

tasks.withType<Javadoc> {
    options.encoding = "UTF-8"
}

tasks.runServer {
    minecraftVersion("1.21.11")
    jvmArgs("-DPaper.IgnoreJavaVersion=true", "-Dcom.mojang.eula.agree=true")
}

publishing {
    publications.create<MavenPublication>("maven") {
        from(components["java"])
    }
}