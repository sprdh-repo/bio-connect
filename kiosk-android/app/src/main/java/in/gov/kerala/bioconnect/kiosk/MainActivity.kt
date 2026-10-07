package `in`.gov.kerala.bioconnect.kiosk

import android.Manifest
import android.app.ActivityManager
import android.app.AlertDialog
import android.app.PendingIntent
import android.content.*
import android.content.pm.PackageManager
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.hardware.usb.UsbDevice
import android.hardware.usb.UsbManager
import android.bluetooth.BluetoothManager
import android.net.Uri
import android.os.*
import android.text.InputType
import android.view.*
import android.webkit.*
import android.widget.*
import androidx.activity.ComponentActivity
import androidx.activity.OnBackPressedCallback
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.core.content.ContextCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.*
import java.io.ByteArrayInputStream

class MainActivity : ComponentActivity() {
    private val model: PrinterViewModel by viewModels()
    private lateinit var root: FrameLayout
    private lateinit var web: WebView
    private var overlay: View? = null
    private var retryJob: Job? = null
    private var adminPoll: Job? = null
    private var lastUrl = SitePolicy.KIOSK
    private var locked = false
    private var bridgeReady = false
    private val taps = ArrayDeque<Long>()
    private var usbReceiver: BroadcastReceiver? = null
    private var nativePermission: PermissionRequest? = null
    private val permissionLauncher = registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        nativePermission?.let { if (granted) it.grant(arrayOf(PermissionRequest.RESOURCE_VIDEO_CAPTURE)) else it.deny() }; nativePermission = null
        if (granted && ::web.isInitialized) web.reload()
    }
    private val networkLauncher = registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (granted) model.refresh() else toast("Allow local network access to use the TCP printer")
    }
    private var bluetoothGranted: (() -> Unit)? = null
    private val bluetoothLauncher = registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (granted) bluetoothGranted?.invoke() else toast("Allow Bluetooth access to choose a printer")
        bluetoothGranted = null
    }
    private val forest = Color.rgb(11, 51, 41)
    private val cream = Color.rgb(245, 243, 235)
    private val ink = Color.rgb(16, 32, 27)
    private val muted = Color.rgb(93, 107, 99)
    private val errorColor = Color.rgb(173, 52, 39)
    private val body by lazy { resources.getFont(R.font.dm_sans) }
    private val heading by lazy {
        val font = resources.getFont(R.font.manrope)
        if (Build.VERSION.SDK_INT >= 28) Typeface.create(font, 800, false) else Typeface.create(font, Typeface.BOLD)
    }
    private fun dp(v: Int) = (resources.displayMetrics.density * v).toInt()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        WindowCompat.setDecorFitsSystemWindows(window, false)
        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) { override fun handleOnBackPressed() {} })
        root = FrameLayout(this).apply { setBackgroundColor(cream) }; setContentView(root)
        createWebView()
        configureOwner(this)
        if (model.settings.hasPin) enterKiosk() else showPinSetup()
        if (ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA) != PackageManager.PERMISSION_GRANTED)
            permissionLauncher.launch(Manifest.permission.CAMERA)
        immersive()
    }
    private fun immersive() {
        WindowInsetsControllerCompat(window, root).apply {
            hide(WindowInsetsCompat.Type.systemBars())
            systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        }
    }
    override fun onWindowFocusChanged(hasFocus: Boolean) { super.onWindowFocusChanged(hasFocus); if (hasFocus && ::root.isInitialized) immersive() }
    override fun onResume() { super.onResume(); if (::root.isInitialized) immersive() }
    override fun onPause() { CookieManager.getInstance().flush(); super.onPause() }
    override fun dispatchTouchEvent(event: MotionEvent): Boolean {
        if (event.action == MotionEvent.ACTION_UP && event.x < dp(96) && event.y < dp(96)) {
            val now = SystemClock.elapsedRealtime(); taps.addLast(now)
            while (taps.isNotEmpty() && now - taps.first() > 3000) taps.removeFirst()
            if (taps.size >= 5) { taps.clear(); if (model.settings.hasPin && overlay == null) showPinEntry() }
        }
        return super.dispatchTouchEvent(event)
    }
    @Suppress("SetJavaScriptEnabled")
    private fun createWebView() {
        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG)
        CookieManager.getInstance().setAcceptCookie(true)
        web = WebView(this).apply {
            setBackgroundColor(cream); overScrollMode = View.OVER_SCROLL_NEVER
            isLongClickable = false; setOnLongClickListener { true }
            setOnCreateContextMenuListener { _, _, _ -> }
            settings.apply {
                javaScriptEnabled = true; domStorageEnabled = true
                mediaPlaybackRequiresUserGesture = false
                allowFileAccess = false; allowContentAccess = false
                @Suppress("DEPRECATION")
                allowFileAccessFromFileURLs = false
                @Suppress("DEPRECATION")
                allowUniversalAccessFromFileURLs = false
                mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
                setSupportZoom(false); builtInZoomControls = false; displayZoomControls = false
                javaScriptCanOpenWindowsAutomatically = false; setSupportMultipleWindows(false)
                cacheMode = WebSettings.LOAD_DEFAULT
            }
            webViewClient = object : WebViewClient() {
                override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest) = !SitePolicy.allows(request.url.toString())
                override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest): WebResourceResponse? {
                    if (SitePolicy.allows(request.url.toString())) return null
                    return WebResourceResponse("text/plain", "UTF-8", 403, "Blocked", emptyMap(), ByteArrayInputStream(ByteArray(0)))
                }
                override fun onReceivedError(view: WebView, request: WebResourceRequest, error: WebResourceError) {
                    if (request.isForMainFrame) showOffline("Check the tablet’s internet connection")
                }
                override fun onReceivedHttpError(view: WebView, request: WebResourceRequest, response: WebResourceResponse) {
                    if (request.isForMainFrame && response.statusCode >= 400) showOffline("The kiosk is temporarily unavailable")
                }
                override fun onPageStarted(view: WebView, url: String, favicon: android.graphics.Bitmap?) {
                    if (!SitePolicy.allows(url)) { view.stopLoading(); showOffline("This address is blocked") }
                    else lastUrl = url
                }
                override fun onPageFinished(view: WebView, url: String) {
                    // Error callbacks own the overlay; success is confirmed by HTTP/navigation completion.
                    if (this@MainActivity.overlay?.tag == "offline" && !loadFailed) hideOverlay()
                    loadFailed = false
                }
                override fun onReceivedSslError(view: WebView, handler: android.webkit.SslErrorHandler, error: android.net.http.SslError) {
                    handler.cancel(); showOffline("The kiosk connection could not be verified")
                }
                override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean {
                    root.removeView(view); view.destroy(); createWebView(); showOffline("The kiosk needs to reload"); return true
                }
            }
            webChromeClient = object : WebChromeClient() {
                override fun onPermissionRequest(request: PermissionRequest) {
                    runOnUiThread {
                        if (!SitePolicy.allows(request.origin.toString()) || !request.resources.contains(PermissionRequest.RESOURCE_VIDEO_CAPTURE)) { request.deny(); return@runOnUiThread }
                        if (ContextCompat.checkSelfPermission(this@MainActivity, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED)
                            request.grant(arrayOf(PermissionRequest.RESOURCE_VIDEO_CAPTURE))
                        else { nativePermission?.deny(); nativePermission = request; permissionLauncher.launch(Manifest.permission.CAMERA) }
                    }
                }
                override fun onPermissionRequestCanceled(request: PermissionRequest) { if (nativePermission === request) nativePermission = null }
                override fun onCreateWindow(view: WebView, isDialog: Boolean, isUserGesture: Boolean, resultMsg: Message) = false
            }
        }
        CookieManager.getInstance().setAcceptThirdPartyCookies(web, false)
        root.addView(web, 0, FrameLayout.LayoutParams(-1, -1))
        bridgeReady = KioskBridge.install(web, model.service, lifecycleScope)
        if (!bridgeReady) showOffline("Update Android System WebView to enable direct printing", false)
    }
    private var loadFailed = false
    private fun enterKiosk() {
        if (!bridgeReady) { hideOverlay(); showOffline("Update Android System WebView to enable direct printing", false); return }
        hideOverlay(); locked = true
        runCatching { startLockTask() }.onFailure { toast("Could not start screen pinning") }
        loadFailed = false; web.loadUrl(model.settings.url)
    }
    private fun retry() { if (!bridgeReady) { toast("Update Android System WebView, then restart the app"); return }; loadFailed = false; web.loadUrl(if (SitePolicy.allows(lastUrl)) lastUrl else model.settings.url) }
    private fun showOffline(message: String, autoRetry: Boolean = true) {
        loadFailed = true
        if (overlay != null && overlay?.tag != "offline") return
        val column = column("Connection interrupted", message)
        button(column, "Retry now") { retry() }
        button(column, "Staff setup") { showPinEntry() }
        showOverlay(column, "offline")
        if (autoRetry) retryJob = lifecycleScope.launch { while (isActive) { delay(10_000); retry() } }
    }
    private fun column(title: String, subtitle: String): LinearLayout = LinearLayout(this).apply {
        orientation = LinearLayout.VERTICAL; setPadding(dp(32), dp(28), dp(32), dp(28))
        addView(text(title, 30, forest, true)); addView(text(subtitle, 18, muted))
    }
    private fun text(value: String, size: Int = 18, color: Int = ink, bold: Boolean = false) = TextView(this).apply {
        text = value; textSize = size.toFloat(); setTextColor(color); typeface = if (bold) heading else body
        setPadding(0, dp(8), 0, dp(12))
    }
    private fun button(parent: LinearLayout, label: String, click: () -> Unit) = Button(this).apply {
        text = label; textSize = 18f; typeface = body; isAllCaps = false; setTextColor(cream)
        background = GradientDrawable().apply { setColor(forest); cornerRadius = dp(40).toFloat() }
        minHeight = dp(64); setPadding(dp(20), dp(12), dp(20), dp(12))
        parent.addView(this, LinearLayout.LayoutParams(-1, -2).apply { topMargin = dp(14); bottomMargin = dp(4) })
        setOnClickListener { click() }
    }
    private fun field(parent: LinearLayout, label: String, value: String = "", numeric: Boolean = false): EditText {
        parent.addView(text(label, 16, muted))
        return EditText(this).apply {
            setText(value); textSize = 20f; typeface = body; setTextColor(ink); minHeight = dp(64)
            isSingleLine = true
            inputType = if (numeric) InputType.TYPE_CLASS_NUMBER or InputType.TYPE_NUMBER_VARIATION_PASSWORD else InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_URI
            minimumHeight = dp(64)
            parent.addView(this, LinearLayout.LayoutParams(-1, -2))
        }
    }
    private fun showOverlay(content: LinearLayout, tag: String) {
        hideOverlay()
        val scroll = ScrollView(this).apply { setBackgroundColor(cream); isFillViewport = true; overScrollMode = View.OVER_SCROLL_NEVER; this.tag = tag }
        val container = LinearLayout(this).apply { gravity = Gravity.CENTER; orientation = LinearLayout.VERTICAL }
        container.addView(content, LinearLayout.LayoutParams(minOf(resources.displayMetrics.widthPixels, dp(800)), -2))
        scroll.addView(container, FrameLayout.LayoutParams(-1, -1))
        overlay = scroll; root.addView(scroll, FrameLayout.LayoutParams(-1, -1))
    }
    private fun hideOverlay() {
        retryJob?.cancel(); retryJob = null; adminPoll?.cancel(); adminPoll = null
        overlay?.let { root.removeView(it) }; overlay = null
    }
    private fun showPinSetup() {
        val c = column("Bio Connect Kiosk", "Set a 6-digit tablet admin PIN. Staff use this PIN for native printer setup.")
        val pin = field(c, "Admin PIN", numeric = true); val confirmation = field(c, "Confirm PIN", numeric = true)
        val message = text("", 16, errorColor); c.addView(message)
        button(c, "Save PIN and set up printer") {
            val value = pin.text.toString()
            if (!value.matches(Regex("[0-9]{6}")) || value != confirmation.text.toString()) message.text = "Enter the same 6-digit PIN in both fields"
            else lifecycleScope.launch {
                withContext(Dispatchers.IO) { model.settings.setPin(value) }
                showAdmin()
            }
        }
        showOverlay(c, "setup")
    }
    private fun showPinEntry() {
        val c = column("Staff access", "Enter the tablet admin PIN.")
        val pin = field(c, "6-digit PIN", numeric = true); val message = text("", 16, errorColor); c.addView(message)
        lateinit var submit: Button
        submit = button(c, "Unlock setup") {
            submit.isEnabled = false
            val value = pin.text.toString()
            lifecycleScope.launch {
                val ok = withContext(Dispatchers.IO) { model.settings.verifyPin(value) }
                submit.isEnabled = true
                if (ok) showAdmin() else { message.text = "Incorrect PIN or temporarily locked. Try again in one minute."; pin.text.clear() }
            }
        }
        button(c, "Return to kiosk") { hideOverlay(); if (!locked) enterKiosk() else retry() }
        showOverlay(c, "pin"); submit.requestFocus()
    }
    private fun showAdmin() {
        val c = column("Tablet setup", "Bio Connect Kiosk ${BuildConfig.VERSION_NAME}")
        val status = text("Checking printer…", 18, muted); c.addView(status)
        val choice = Spinner(this)
        val options = arrayOf("No printer", "Bluetooth Classic", "USB", "TCP / IP (port 9100)")
        val codes = arrayOf("none", "bluetooth", "usb", "tcp")
        choice.adapter = ArrayAdapter(this, android.R.layout.simple_spinner_dropdown_item, options)
        choice.minimumHeight = dp(64); c.addView(text("Connection type", 16, muted)); c.addView(choice)
        choice.setSelection(codes.indexOf(model.settings.printer.connection).coerceAtLeast(0))
        val address = field(c, "Printer device or IPv4 address", model.settings.printer.address)
        button(c, "Choose paired / connected printer") {
            when (codes[choice.selectedItemPosition]) {
                "bluetooth" -> chooseBluetooth { address.setText(it) }
                "usb" -> chooseUsb { address.setText(it) }
                else -> toast("Select Bluetooth or USB, or enter the printer IP address")
            }
        }
        c.addView(text("Resolution", 16, muted))
        val dpi = Spinner(this).apply {
            adapter = ArrayAdapter(this@MainActivity, android.R.layout.simple_spinner_dropdown_item, arrayOf("Detect automatically", "203 dpi", "300 dpi"))
            minimumHeight = dp(64)
            setSelection(when (model.settings.printer.dpiOverride) { 203 -> 1; 300 -> 2; else -> 0 })
        }; c.addView(dpi)
        val url = field(c, "Kiosk URL (approved site only)", model.settings.url)
        val message = text("", 16, muted); c.addView(message)
        fun save(): Boolean {
            if (!SitePolicy.allows(url.text.toString())) { message.text = "Use an HTTPS URL on reg.bioconnect.kerala.gov.in"; return false }
            model.settings.url = url.text.toString()
            model.settings.printer = PrinterConfig(codes[choice.selectedItemPosition], address.text.toString().trim(), when (dpi.selectedItemPosition) { 1 -> 203; 2 -> 300; else -> 0 })
            if (codes[choice.selectedItemPosition] == "tcp" && Build.VERSION.SDK_INT >= 37 &&
                ContextCompat.checkSelfPermission(this, Manifest.permission.ACCESS_LOCAL_NETWORK) != PackageManager.PERMISSION_GRANTED) {
                networkLauncher.launch(Manifest.permission.ACCESS_LOCAL_NETWORK)
                message.text = "Allow local network access, then try this action again"
                return false
            }
            return true
        }
        button(c, "Save and check printer") { if (save()) model.refresh() }
        button(c, "Print test label") { if (save()) model.testLabel { message.text = it } }
        button(c, "Calibrate media") { if (save()) lifecycleScope.launch { message.text = model.service.calibrate() } }
        button(c, "Open staff web setup") { if (save()) { if (!bridgeReady) toast("Update Android System WebView, then restart the app") else { hideOverlay(); web.loadUrl("${SitePolicy.ORIGIN}/ops") } } }
        button(c, "Clear session") {
            AlertDialog.Builder(this).setTitle("Clear staff session?").setMessage("Staff will need to sign in again on this tablet.")
                .setNegativeButton("Cancel", null).setPositiveButton("Clear") { _, _ ->
                    web.stopLoading(); CookieManager.getInstance().removeAllCookies { CookieManager.getInstance().flush(); WebStorage.getInstance().deleteAllData(); web.clearCache(true); message.text = "Session cleared" }
                }.show()
        }
        button(c, "Exit kiosk lock") { stopLockTask(); locked = false; message.text = "Kiosk lock stopped. Tap Start kiosk to lock again." }
        button(c, "Start kiosk") { if (save()) enterKiosk() }
        showOverlay(c, "admin")
        adminPoll = lifecycleScope.launch {
            launch { model.state.collect { s -> status.text = "${s.name} · ${s.dpi} dpi · ${s.connection}\nDetected resolution: ${s.detectedDpi?.let { "$it dpi" } ?: "unavailable"}\n${if (s.ready) "Ready" else s.problem}" } }
            while (isActive) { model.service.refresh(); delay(5000) }
        }
    }
    private fun chooseBluetooth(selected: (String) -> Unit) {
        if (Build.VERSION.SDK_INT >= 31 && ContextCompat.checkSelfPermission(this, Manifest.permission.BLUETOOTH_CONNECT) != PackageManager.PERMISSION_GRANTED) {
            bluetoothGranted = { chooseBluetooth(selected) }; bluetoothLauncher.launch(Manifest.permission.BLUETOOTH_CONNECT); return
        }
        try {
            val adapter = getSystemService(BluetoothManager::class.java).adapter
            if (adapter == null || !adapter.isEnabled) { toast("Turn on Bluetooth in tablet settings"); return }
            @Suppress("MissingPermission") val devices = adapter.bondedDevices.toList()
            if (devices.isEmpty()) { toast("Pair the Zebra printer in Android Bluetooth settings first"); return }
            @Suppress("MissingPermission") val names = devices.map { "${it.name ?: "Printer"} (${it.address})" }.toTypedArray()
            AlertDialog.Builder(this).setTitle("Paired printers").setItems(names) { _, index -> selected(devices[index].address) }.show()
        } catch (_: SecurityException) { toast("Allow Bluetooth access in tablet settings") }
    }
    private fun chooseUsb(selected: (String) -> Unit) {
        val manager = getSystemService(UsbManager::class.java)
        val devices = manager.deviceList.values.filter { it.vendorId == 0x0a5f || (0 until it.interfaceCount).any { n -> it.getInterface(n).interfaceClass == android.hardware.usb.UsbConstants.USB_CLASS_PRINTER } }
        if (devices.isEmpty()) { toast("Connect the Zebra printer with a USB OTG cable"); return }
        AlertDialog.Builder(this).setTitle("USB printers").setItems(devices.map { it.productName ?: UsbTransport.usbKey(it) }.toTypedArray()) { _, index ->
            val d = devices[index]
            if (manager.hasPermission(d)) selected(UsbTransport.usbKey(d)) else requestUsb(manager, d, selected)
        }.show()
    }
    private fun requestUsb(manager: UsbManager, device: UsbDevice, selected: (String) -> Unit) {
        usbReceiver?.let { unregisterReceiver(it) }
        val action = "$packageName.USB_PERMISSION"
        usbReceiver = object : BroadcastReceiver() {
            override fun onReceive(context: Context, intent: Intent) {
                if (intent.action != action) return
                if (manager.hasPermission(device)) selected(UsbTransport.usbKey(device)) else toast("USB permission was denied")
                unregisterReceiver(this); usbReceiver = null
            }
        }
        ContextCompat.registerReceiver(this, usbReceiver, IntentFilter(action), ContextCompat.RECEIVER_NOT_EXPORTED)
        val pending = PendingIntent.getBroadcast(this, 0, Intent(action).setPackage(packageName), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
        manager.requestPermission(device, pending)
    }
    private fun toast(message: String) { Toast.makeText(this, message, Toast.LENGTH_LONG).show() }
    override fun onDestroy() {
        nativePermission?.deny(); usbReceiver?.let { unregisterReceiver(it) }; usbReceiver = null
        retryJob?.cancel(); adminPoll?.cancel()
        if (::web.isInitialized) { web.stopLoading(); root.removeView(web); web.destroy() }
        super.onDestroy()
    }
}
