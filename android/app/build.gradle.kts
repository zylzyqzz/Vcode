plugins { id("com.android.application") }

android { namespace = "xin.aimj.vcode"; compileSdk = 35
    defaultConfig { applicationId = "xin.aimj.vcode"; minSdk = 26; targetSdk = 35; versionCode = 1; versionName = "0.1.0" }
    signingConfigs {
        create("release") {
            storeFile = file(System.getenv("VCODE_SIGNING_STORE") ?: "${System.getProperty("user.home")}/.android/debug.keystore")
            storePassword = "android"
            keyAlias = "androiddebugkey"
            keyPassword = "android"
        }
    }
    buildTypes {
        release {
            isMinifyEnabled = false
            isDebuggable = false
            signingConfig = signingConfigs.getByName("release")
        }
    }
}
