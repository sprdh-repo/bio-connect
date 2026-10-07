plugins { id("com.android.application") }
android {
    namespace = "in.gov.kerala.bioconnect.kiosk"
    compileSdk { version = release(37) { minorApiLevel = 0 } }
    defaultConfig {
        applicationId = "in.gov.kerala.bioconnect.kiosk"
        minSdk = 26
        targetSdk = 37
        versionCode = 1
        versionName = "1.0.0"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }
    val store = providers.environmentVariable("KIOSK_KEYSTORE").orNull
    signingConfigs {
        if (store != null) create("release") {
            storeFile = file(store)
            storePassword = providers.environmentVariable("KIOSK_STORE_PASSWORD").get()
            keyAlias = providers.environmentVariable("KIOSK_KEY_ALIAS").get()
            keyPassword = providers.environmentVariable("KIOSK_KEY_PASSWORD").get()
        }
    }
    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            if (store != null) signingConfig = signingConfigs.getByName("release")
        }
    }
    buildFeatures { buildConfig = true }
    compileOptions { sourceCompatibility = JavaVersion.VERSION_17; targetCompatibility = JavaVersion.VERSION_17 }
    testOptions { unitTests.isReturnDefaultValues = false }
}
dependencies {
    implementation("androidx.activity:activity-ktx:1.13.0")
    implementation("androidx.lifecycle:lifecycle-viewmodel-ktx:2.10.0")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.10.0")
    implementation("androidx.webkit:webkit:1.15.0")
    implementation("androidx.security:security-crypto:1.1.0")
    // Tink references these annotation types; provide them to R8 instead of disabling missing-class checks.
    implementation("com.google.errorprone:error_prone_annotations:2.36.0")
    implementation("com.google.code.findbugs:jsr305:3.0.2")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.10.2")
    testImplementation("junit:junit:4.13.2")
    testImplementation("org.json:json:20250517")
    androidTestImplementation("androidx.test:runner:1.6.2")
    androidTestImplementation("androidx.test:core-ktx:1.6.1")
    androidTestImplementation("androidx.test.ext:junit:1.2.1")
}
