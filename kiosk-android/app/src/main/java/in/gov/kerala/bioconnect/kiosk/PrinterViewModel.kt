package `in`.gov.kerala.bioconnect.kiosk

import android.app.Application
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.util.Base64
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.ByteArrayOutputStream
import java.util.UUID

class PrinterViewModel(application: Application) : AndroidViewModel(application) {
    val settings = Settings(application)
    val service = PrinterService(application, { settings.printer })
    val state = service.state
    fun refresh() { viewModelScope.launch { service.refresh() } }
    fun testLabel(callback: (String) -> Unit) { viewModelScope.launch {
        val dpi = service.refresh().dpi
        val job = withContext(Dispatchers.Default) {
            val w = if (dpi == 203) 608 else 900
            val h = if (dpi == 203) 406 else 600
            val b = Bitmap.createBitmap(w, h, Bitmap.Config.ARGB_8888)
            val c = Canvas(b); c.drawColor(Color.WHITE)
            val paint = Paint().apply { color = Color.BLACK; strokeWidth = 3f; style = Paint.Style.STROKE }
            c.drawRect(12f, 12f, w - 12f, h - 12f, paint)
            paint.style = Paint.Style.FILL; paint.textSize = w / 25f; paint.isAntiAlias = false
            c.drawText("BIO CONNECT TEST ${dpi}dpi", 28f, 65f, paint)
            for (row in 0..7) for (col in 0..15) if ((row + col) % 2 == 0) c.drawRect(28f + col * 12, 100f + row * 12, 40f + col * 12, 112f + row * 12, paint)
            c.drawText("One label - 3 x 2 inches", 28f, h - 40f, paint)
            val out = ByteArrayOutputStream(); b.compress(Bitmap.CompressFormat.PNG, 100, out); b.recycle()
            PageMessage.Print(UUID.randomUUID().toString(), Base64.encodeToString(out.toByteArray(), Base64.NO_WRAP), w, h)
        }
        callback(service.print(job).getString("message"))
    } }
    override fun onCleared() { service.close() }
}
