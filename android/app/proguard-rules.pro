# The Go decryption core reaches JavaScript through js/wasm glue that relies on
# reflection-free but dynamically named globals. Nothing in the app is reached
# by name from Java, so the defaults in proguard-android-optimize.txt are enough;
# the rules below only pin what Compose, Kotlin and the WASM engine rely on.

# Keep line numbers for readable crash reports, but hide the original file name.
-keepattributes SourceFile,LineNumberTable
-renamesourcefileattribute SourceFile

# Kotlin metadata is required for reflection-free intrinsics used by Compose.
-keepclassmembers class ** {
    @kotlinx.coroutines.internal.InlineOnly <methods>;
}

# Compose keeps its own consumer rules; silence warnings for optional integrations.
-dontwarn org.jetbrains.annotations.**
-dontwarn kotlinx.serialization.**

# The engine page is plain ES2020 without minification; nothing to keep.
