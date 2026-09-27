package io.github.frontiertm.pantegnos

import android.annotation.SuppressLint
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.res.Configuration
import android.graphics.Color
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.util.Base64
import android.util.Log
import android.view.HapticFeedbackConstants
import android.view.View
import android.view.ViewGroup
import android.view.ViewGroup.LayoutParams.MATCH_PARENT
import android.webkit.ConsoleMessage
import android.webkit.JavascriptInterface
import android.webkit.WebChromeClient
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.annotation.MainThread
import androidx.core.content.FileProvider
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.webkit.WebViewAssetLoader
import java.io.ByteArrayInputStream
import java.io.File
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import org.json.JSONArray
import org.json.JSONObject

/**
 * The entire Android shell.
 *
 * All behaviour and the whole interface live in Go: `internal/mobile` renders the
 * markup, runs the queue, asks for passphrases and translates the strings, compiled to
 * `js/wasm`. This class only provides the handful of things the platform reserves
 * for the app process - the document picker, the share sheet, the clipboard, the
 * system bars and persisted preferences - and forwards them to Go over
 * `window.PantegnosHost`.
 *
 * The WebView is parented to a 1x1, fully transparent host: the renderer needs a
 * window, but the user never sees it.
 */
class MainActivity : ComponentActivity() {

    private var webView: WebView? = null
    private var pendingSave: Pair<String, String>? = null

    private val prefs by lazy {
        getSharedPreferences("pantegnos_prefs", Context.MODE_PRIVATE)
    }

    private val pickFiles = registerForActivityResult(
        ActivityResultContracts.OpenMultipleDocuments(),
    ) { uris -> deliver(uris) }

    private val saveDocument = registerForActivityResult(
        ActivityResultContracts.CreateDocument("text/plain"),
    ) { uri -> writePendingSave(uri) }

    /** Holds the file picker open while Go blocks on the JavaScript call. */
    private var pickLatch = CountDownLatch(0)
    @Volatile private var pickedJSON = "[]"

    private var saveLatch = CountDownLatch(0)
    @Volatile private var savedOK = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.auto(Color.TRANSPARENT, Color.TRANSPARENT) { _ -> dark() },
            navigationBarStyle = SystemBarStyle.auto(Color.TRANSPARENT, Color.TRANSPARENT) { _ -> dark() },
        )

        val container = FrameLayout(this)
        setContentView(container)

        // The WebView is the interface, so it fills the window. It is still
        // offline: the page and the core are the only things it can load, the
        // client refuses every other URL, and there is no INTERNET permission.
        val web = createWebView()
        container.addView(web, FrameLayout.LayoutParams(MATCH_PARENT, MATCH_PARENT))
        webView = web
    }

    private fun createWebView(): WebView {
        val loader = WebViewAssetLoader.Builder()
            .setDomain(ASSET_DOMAIN)
            .addPathHandler("/assets/", WebViewAssetLoader.AssetsPathHandler(this))
            .build()

        return WebView(this).apply {
            @SuppressLint("SetJavaScriptEnabled")
            settings.apply {
                javaScriptEnabled = true
                domStorageEnabled = false
                allowFileAccess = false
                allowContentAccess = false
                cacheMode = WebSettings.LOAD_NO_CACHE
                setSupportMultipleWindows(false)
                javaScriptCanOpenWindowsAutomatically = false
                mediaPlaybackRequiresUserGesture = true
                setGeolocationEnabled(false)
            }
            setBackgroundColor(Color.TRANSPARENT)
            overScrollMode = View.OVER_SCROLL_NEVER
            isVerticalScrollBarEnabled = false
            isHorizontalScrollBarEnabled = false
            addJavascriptInterface(HostBridge(), "PantegnosHost")
            webChromeClient = object : WebChromeClient() {
                // The interface runs inside the page, so its console is the only
                // place a boot failure can surface. Mirror it to logcat.
                override fun onConsoleMessage(message: ConsoleMessage): Boolean {
                    Log.d(
                        TAG,
                        "${message.message()} (${message.sourceId()}:${message.lineNumber()})",
                    )
                    return true
                }
            }
            webViewClient = object : WebViewClient() {
                // Serve the bundled page and refuse everything else, so this
                // WebView can never behave like a general purpose browser. The
                // app declares no INTERNET permission either.
                override fun shouldInterceptRequest(
                    view: WebView,
                    request: WebResourceRequest,
                ): WebResourceResponse = loader.shouldInterceptRequest(request.url) ?: blocked()

                override fun shouldOverrideUrlLoading(
                    view: WebView,
                    request: WebResourceRequest,
                ): Boolean = true

                override fun onReceivedError(
                    view: WebView,
                    request: WebResourceRequest,
                    error: WebResourceError,
                ) {
                    if (request.isForMainFrame) {
                        Log.e(TAG, "engine page failed: ${error.description}")
                    }
                }
            }
            loadUrl("https://$ASSET_DOMAIN/assets/index.html")
        }
    }

    private fun blocked(): WebResourceResponse = WebResourceResponse(
        "text/plain",
        "utf-8",
        403,
        "Blocked",
        mapOf("Cache-Control" to "no-store"),
        ByteArrayInputStream(ByteArray(0)),
    )

    // -- system bars --------------------------------------------------------

    private fun dark(): Boolean = (resources.configuration.uiMode and
        Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES

    @MainThread
    private fun applyBars(dark: Boolean) {
        val controller = WindowCompat.getInsetsController(window, window.decorView)
        val light = !dark
        controller.isAppearanceLightStatusBars = light
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            controller.isAppearanceLightNavigationBars = light
        }
    }

    // -- file picker --------------------------------------------------------

    @MainThread
    private fun deliver(uris: List<Uri>) {
        val array = JSONArray()
        for (uri in uris) {
            val data = readBounded(uri) ?: continue
            array.put(
                JSONObject()
                    .put("name", displayName(uri))
                    .put("b64", Base64.encodeToString(data, Base64.NO_WRAP)),
            )
        }
        pickedJSON = array.toString()
        pickLatch.countDown()
    }

    /** Reads a picked file, refusing anything the engine would choke on anyway. */
    private fun readBounded(uri: Uri): ByteArray? = runCatching {
        contentResolver.openInputStream(uri)?.use { stream ->
            val sink = java.io.ByteArrayOutputStream(DEFAULT_BUFFER_SIZE)
            val chunk = ByteArray(16 * 1024)
            var total = 0
            while (true) {
                val read = stream.read(chunk)
                if (read < 0) break
                total += read
                if (total > MAX_FILE_BYTES) return null
                sink.write(chunk, 0, read)
            }
            sink.toByteArray()
        }
    }.getOrNull()

    private fun displayName(uri: Uri): String {
        val fromProvider = runCatching {
            contentResolver.query(uri, arrayOf(android.provider.OpenableColumns.DISPLAY_NAME), null, null, null)
                ?.use { cursor ->
                    val index = cursor.getColumnIndex(android.provider.OpenableColumns.DISPLAY_NAME)
                    if (index >= 0 && cursor.moveToFirst()) cursor.getString(index) else null
                }
        }.getOrNull()
        return fromProvider ?: uri.lastPathSegment.orEmpty().ifBlank { "config" }
    }

    private fun writePendingSave(uri: Uri?) {
        val pending = pendingSave
        pendingSave = null
        savedOK = false
        if (uri != null && pending != null) {
            savedOK = runCatching {
                contentResolver.openOutputStream(uri)?.use { it.write(pending.second.toByteArray()) }
            }.getOrNull() != null
        }
        saveLatch.countDown()
    }

    // -- the bridge Go calls -----------------------------------------------

    private inner class HostBridge {

        /** Blocks the JavaScript thread while the picker is open. */
        @JavascriptInterface
        fun pickFiles(): String {
            pickLatch = CountDownLatch(1)
            runOnUiThread { pickFiles.launch(arrayOf("*/*")) }
            pickLatch.await(10, TimeUnit.MINUTES)
            return pickedJSON
        }

        @JavascriptInterface
        fun readClipboard(): String = runCatching {
            val manager = getSystemService(Context.CLIPBOARD_SERVICE) as? ClipboardManager
            val clip = manager?.primaryClip ?: return ""
            if (clip.itemCount == 0) return ""
            clip.getItemAt(0).coerceToText(this@MainActivity)?.toString().orEmpty()
        }.getOrDefault("")

        @JavascriptInterface
        fun copyToClipboard(text: String): Boolean = runCatching {
            val manager = getSystemService(Context.CLIPBOARD_SERVICE) as? ClipboardManager
                ?: return false
            manager.setPrimaryClip(ClipData.newPlainText("Pantegnos", text))
            true
        }.getOrDefault(false)

        @JavascriptInterface
        fun share(name: String, text: String) {
            runOnUiThread { shareConfig(name, text) }
        }

        @JavascriptInterface
        fun save(name: String, text: String) {
            runOnUiThread {
                pendingSave = name to text
                saveLatch = CountDownLatch(1)
                runCatching { saveDocument.launch(name) }
                    .onFailure { saveLatch.countDown() }
            }
        }

        @JavascriptInterface
        fun openExternal(target: String) {
            runOnUiThread { openTarget(target) }
        }

        @JavascriptInterface
        fun loadPrefs(): String = prefs.getString(KEY_PREFS, "").orEmpty()

        @JavascriptInterface
        fun savePrefs(raw: String) {
            prefs.edit().putString(KEY_PREFS, raw).apply()
        }

        @JavascriptInterface
        fun setDark(dark: Boolean) = runOnUiThread { applyBars(dark) }

        @JavascriptInterface
        fun haptic() = runOnUiThread {
            webView?.performHapticFeedback(HapticFeedbackConstants.LONG_PRESS)
        }

        @JavascriptInterface
        fun locale(): String {
            val tag = resources.configuration.locales.get(0).language
            return if (tag == "fa") "fa" else "en"
        }
    }

    // -- share / external links -------------------------------------------

    private fun shareConfig(name: String, text: String) {
        val uri = runCatching { stageForShare(name, text) }.getOrNull()
        if (uri == null) {
            shareTextOnly(text)
            return
        }
        val intent = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(Intent.EXTRA_TEXT, text)
            putExtra(Intent.EXTRA_SUBJECT, name)
            putExtra(Intent.EXTRA_STREAM, uri)
            // Carrying the URI on the clip lets receivers that expect a file pick
            // it up, while everyone else just uses the text.
            clipData = ClipData.newRawUri("Pantegnos", uri)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
        runCatching {
            startActivity(Intent.createChooser(intent, getString(R.string.action_share)))
        }.onFailure { shareTextOnly(text) }
    }

    private fun shareTextOnly(text: String) {
        val intent = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(Intent.EXTRA_TEXT, text)
        }
        runCatching { startActivity(Intent.createChooser(intent, null)) }
    }

    private fun stageForShare(name: String, text: String): Uri {
        val dir = File(cacheDir, "shared").apply { mkdirs() }
        dir.listFiles()?.forEach { stale ->
            if (System.currentTimeMillis() - stale.lastModified() > CACHE_TTL_MS) stale.delete()
        }
        val file = File(dir, sanitizeForFile(name))
        file.writeText(text)
        return FileProvider.getUriForFile(this, "$packageName.fileprovider", file)
    }

    private fun sanitizeForFile(name: String): String {
        val cleaned = name.substringAfterLast('/').substringAfterLast('\\').trim()
        return cleaned.take(120).ifBlank { "pantegnos.txt" }
    }

    private fun openTarget(target: String) {
        val candidates = when {
            target.contains("://") -> listOf(Uri.parse(target))
            target.isNotBlank() -> listOf(
                Uri.parse("market://details?id=$target"),
                Uri.parse("https://play.google.com/store/apps/details?id=$target"),
            )
            else -> emptyList()
        }
        for (uri in candidates) {
            val intent = Intent(Intent.ACTION_VIEW, uri)
            if (intent.resolveActivity(packageManager) != null) {
                runCatching { startActivity(intent) }
                return
            }
        }
    }

    override fun onDestroy() {
        webView?.let { view ->
            (view.parent as? ViewGroup)?.removeView(view)
            view.stopLoading()
            view.destroy()
        }
        webView = null
        super.onDestroy()
    }

    private companion object {
        const val TAG = "PantegnosWeb"
        const val ASSET_DOMAIN = "appassets.androidplatform.net"
        const val KEY_PREFS = "settings"
        const val MAX_FILE_BYTES = 4 * 1024 * 1024
        const val CACHE_TTL_MS = 60L * 60L * 1000L
    }
}
