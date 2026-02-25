plugins {
    `java-library`
    `maven-publish`
    alias(libs.plugins.shadow)
    alias(libs.plugins.runpaper)
}

repositories {
    mavenLocal()
    mavenCentral()
    maven("https://jitpack.io")
    maven("https://repo.papermc.io/repository/maven-public/")
    maven("https://repo.maven.apache.org/maven2/")
}

dependencies {
    compileOnly(libs.paper)
    compileOnly(libs.vault)
    api(libs.websocket)
    compileOnly(libs.log4j)
}

group = "net.trybeacon"
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

    downloadPlugins {
        github("MilkBowl", "Vault", "1.7.3", "Vault.jar")
        modrinth("luckperms", "OrIs0S6b")
    }
}

publishing {
    publications.create<MavenPublication>("maven") {
        from(components["java"])
    }
}