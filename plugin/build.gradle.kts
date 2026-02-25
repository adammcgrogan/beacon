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

val backendProjectDir = projectDir.parentFile.resolve("backend")

data class GoBuildTarget(
    val goos: String,
    val goarch: String
) {
    val filename: String
        get() = "beacon-backend-$goos-$goarch" + if (goos == "windows") ".exe" else ""

    val taskNameSuffix: String
        get() = goos.replaceFirstChar(Char::uppercaseChar) + goarch.replaceFirstChar(Char::uppercaseChar)
}

val goBuildTargets = listOf(
    GoBuildTarget("linux", "amd64"),
    GoBuildTarget("linux", "arm64"),
    GoBuildTarget("darwin", "amd64"),
    GoBuildTarget("darwin", "arm64"),
    GoBuildTarget("windows", "amd64")
)

val backendBuildOutputDir = layout.buildDirectory.dir("generated/backend-targets")

val buildGoBackendTasks = goBuildTargets.map { target ->
    val outputFile = backendBuildOutputDir.map { it.file(target.filename) }
    tasks.register<Exec>("buildGoBackend${target.taskNameSuffix}") {
        group = "build"
        description = "Builds Beacon Go backend for ${target.goos}/${target.goarch}."
        workingDir = backendProjectDir
        environment("GOOS", target.goos)
        environment("GOARCH", target.goarch)
        environment("CGO_ENABLED", "0")
        commandLine("go", "build", "-o", outputFile.get().asFile.absolutePath, "./cmd/server")
        outputs.file(outputFile)
        notCompatibleWithConfigurationCache("Exec task invokes local Go toolchain and is configured dynamically.")

        doFirst {
            outputFile.get().asFile.parentFile.mkdirs()
        }
    }
}

tasks.processResources {
    dependsOn(buildGoBackendTasks)
    from(backendBuildOutputDir) {
        into("backend")
    }
}

publishing {
    publications.create<MavenPublication>("maven") {
        from(components["java"])
    }
}