import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
}

// The version comes from the workflow, never from a literal here. A local build
// with no -PversionName, and any build where the property is empty, is "dev".
val appVersion: String = (findProperty("versionName") as String?)?.trim()?.takeIf { it.isNotEmpty() }
    ?: "dev"

android {
    namespace = "io.github.frontiertm.pantegnos"
    compileSdk = 35

    defaultConfig {
        applicationId = "io.github.frontiertm.pantegnos"
        minSdk = 26
        targetSdk = 35
        versionCode = (findProperty("versionCode") as String?)?.toInt() ?: 1
        versionName = appVersion

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        vectorDrawables.useSupportLibrary = true

        buildConfigField("String", "VERSION_NAME", "\"$appVersion\"")
    }

    signingConfigs {
        create("release") {
            val store = (findProperty("PANTEGNOS_KEYSTORE") as String?)?.takeIf { it.isNotBlank() }
            if (store != null) {
                storeFile = rootProject.file(store)
                storePassword = (findProperty("PANTEGNOS_STORE_PASSWORD") as String?) ?: ""
                keyAlias = (findProperty("PANTEGNOS_KEY_ALIAS") as String?) ?: "pantegnos"
                keyPassword = (findProperty("PANTEGNOS_KEY_PASSWORD") as String?) ?: ""
            }
        }
    }

    buildTypes {
        debug {
            applicationIdSuffix = ".debug"
            versionNameSuffix = "-debug"
            isMinifyEnabled = false
        }
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
            val keystore = (findProperty("PANTEGNOS_KEYSTORE") as String?)?.takeIf { it.isNotBlank() }
            signingConfig = if (keystore != null) {
                signingConfigs.getByName("release")
            } else {
                // No upload key configured: fall back to the debug key so CI still emits an
                // installable APK. Production builds pass PANTEGNOS_KEYSTORE_* instead.
                logger.warn(
                    "No PANTEGNOS_KEYSTORE provided - release APK will be signed with the debug key. " +
                        "Set PANTEGNOS_KEYSTORE / PANTEGNOS_STORE_PASSWORD / PANTEGNOS_KEY_ALIAS / " +
                        "PANTEGNOS_KEY_PASSWORD for a publishable signature."
                )
                signingConfigs.getByName("debug")
            }
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        buildConfig = true
    }

    lint {
        abortOnError = false
        checkReleaseBuilds = false
        warningsAsErrors = false
    }

    packaging {
        resources {
            excludes += setOf(
                "/META-INF/{AL2.0,LGPL2.1}",
                "/META-INF/DEPENDENCIES",
                "/META-INF/versions/9/previous-compilation-data.bin",
                "DebugProbesKt.bin",
                "kotlin-tooling-metadata.json",
            )
        }
    }

    testOptions {
        unitTests {
            isReturnDefaultValues = true
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

dependencies {
    // The whole application is Go. These are only the platform bridges the
    // shell needs: WebViewAssetLoader to serve the page offline, FileProvider to
    // hand a decrypted file to another app, and the activity/result plumbing.
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity)
    implementation(libs.androidx.webkit)

    testImplementation(libs.junit)
}

// ---------------------------------------------------------------------------
// The application is Go.
//
// `./cmd/mobile` is the whole app: the queue, the redaction, the translations
// and the entire interface, compiled to js/wasm. The Android side contributes a
// WebView plus a handful of platform bridges, so the build stages three files:
// the shell page (committed, in internal/mobile), the Go runtime glue (from
// GOROOT) and the compiled core.
// ---------------------------------------------------------------------------

val repoRoot: java.io.File = rootProject.projectDir.parentFile
val assetsDir: java.io.File = layout.projectDirectory.dir("src/main/assets").asFile
val wasmOutput: java.io.File = File(assetsDir, "pantegnos.wasm")
val shellSource: java.io.File = File(repoRoot, "internal/mobile/index.html")
val shellOutput: java.io.File = File(assetsDir, "index.html")
val wasmExecOutput: java.io.File = File(assetsDir, "wasm_exec.js")

/**
 * Whether the Go toolchain is on PATH, decided by scanning PATH rather than by
 * running `go version`: `providers.exec` throws outright when the executable
 * cannot be started, and `isIgnoreExitValue` only covers a non-zero exit.
 */
val goOnPath: Provider<Boolean> = providers.environmentVariable("PATH").map { pathEnv ->
    val executable = if (System.getProperty("os.name").orEmpty().startsWith("Windows")) {
        "go.exe"
    } else {
        "go"
    }
    pathEnv.split(File.pathSeparator).any { entry -> File(entry, executable).isFile }
}

/**
 * GOROOT, resolved lazily. Starting a process during configuration is not
 * allowed with the configuration cache on, so this is a value source - and it
 * is only ever queried by tasks that already know Go is present.
 */
val goRoot: Provider<String> = providers.exec {
    commandLine("go", "env", "GOROOT")
    isIgnoreExitValue = true
}.standardOutput.asText.map { text -> text.trim() }

/**
 * Locates the Go WebAssembly runtime glue, which moved between Go releases.
 * An empty string means "not found", which stages nothing.
 */
val locateWasmRuntime: (String) -> Any = { root ->
    if (root.isEmpty()) {
        ""
    } else {
        listOf(
            File(root, "lib/wasm/wasm_exec.js"),
            File(root, "misc/wasm/wasm_exec.js"),
        ).firstOrNull { it.isFile } ?: ""
    }
}

val wasmExecSource: Provider<Any> = goRoot.map(locateWasmRuntime)

val buildWasm = tasks.register<Exec>("buildWasm") {
    group = "pantegnos"
    description = "Compiles ./cmd/mobile to WebAssembly: the entire Android application."

    val goAvailable = goOnPath
    val version = appVersion

    workingDir = repoRoot
    inputs.files(
        fileTree(File(repoRoot, "cmd/mobile")) { include("**/*.go") },
        fileTree(File(repoRoot, "internal")) { include("**/*.go") },
        File(repoRoot, "go.mod"),
        File(repoRoot, "go.sum"),
    )
    inputs.property("version", version)
    outputs.file(wasmOutput)
    environment("GOOS", "js")
    environment("GOARCH", "wasm")
    environment("CGO_ENABLED", "0")
    commandLine(
        "go", "build",
        "-trimpath",
        "-ldflags", "-s -w -X main.version=$version",
        "-o", wasmOutput.absolutePath,
        "./cmd/mobile",
    )
    onlyIf { goAvailable.get() }
}

val stageShell = tasks.register<Copy>("stageShell") {
    group = "pantegnos"
    description = "Copies the Go-owned shell page into the app assets."
    from(shellSource)
    into(assetsDir)
}

val stageWasmRuntime = tasks.register<Copy>("stageWasmRuntime") {
    group = "pantegnos"
    description = "Copies the Go WebAssembly runtime glue out of GOROOT into app assets."

    val goAvailable = goOnPath
    val source = wasmExecSource

    onlyIf { goAvailable.get() }
    from(source)
    into(assetsDir)
    rename { "wasm_exec.js" }
}

val verifyWasmAssets = tasks.register("verifyWasmAssets") {
    group = "pantegnos"
    description = "Fails fast when the engine assets are missing from app/src/main/assets."

    val wasm = wasmOutput
    val runtime = wasmExecOutput
    val shell = shellOutput
    val goAvailable = goOnPath
    val goHome = goRoot

    // Verification has to observe the staged files, so it runs after them even
    // though the merge task pulls in all three.
    mustRunAfter(buildWasm, stageShell, stageWasmRuntime)

    doLast {
        val missing = listOf(wasm, runtime, shell).filterNot { it.isFile }
        if (missing.isEmpty()) return@doLast

        val detail = when {
            !goAvailable.get() -> "The Go toolchain is not on PATH."
            missing.any { it == runtime } -> {
                val root = goHome.get()
                if (root.isEmpty()) {
                    "Go is on PATH but `go env GOROOT` returned nothing."
                } else {
                    "Go resolved to GOROOT=$root, which has no lib/wasm/wasm_exec.js."
                }
            }
            else -> "A staging task did not run."
        }
        throw GradleException(
            "Missing engine assets: ${missing.joinToString { it.name }}. $detail\n" +
                "Install Go 1.26+ and re-run the build, or run " +
                "`./gradlew buildWasm stageShell stageWasmRuntime` explicitly."
        )
    }
}

tasks.named("preBuild").configure { dependsOn(buildWasm, stageShell, stageWasmRuntime) }

tasks.matching { it.name.startsWith("merge") && it.name.endsWith("Assets") }.configureEach {
    dependsOn(buildWasm, stageShell, stageWasmRuntime, verifyWasmAssets)
}
