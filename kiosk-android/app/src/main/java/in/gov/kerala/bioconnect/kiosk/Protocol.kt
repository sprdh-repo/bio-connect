package `in`.gov.kerala.bioconnect.kiosk

import org.json.JSONObject
import java.net.URI
import java.util.UUID

object SitePolicy {
    const val ORIGIN = "https://reg.bioconnect.kerala.gov.in"
    const val KIOSK = "$ORIGIN/kiosk"
    fun allows(url: String): Boolean = runCatching {
        val u = URI(url)
        u.scheme.equals("https", true) && u.host.equals("reg.bioconnect.kerala.gov.in", true) &&
            (u.port == -1 || u.port == 443) && u.rawUserInfo == null
    }.getOrDefault(false)
}

sealed interface PageMessage {
    data object Hello : PageMessage
    data object Status : PageMessage
    data class Print(val jobId: String, val png: String, val width: Int, val height: Int) : PageMessage
    companion object {
        fun parse(raw: String): PageMessage {
            require(raw.length <= 2_500_000) { "Badge image is too large" }
            val j = JSONObject(raw)
            return when (j.getString("type")) {
                "hello" -> Hello
                "status" -> Status
                "print" -> {
                    val id = j.getString("jobId")
                    require(UUID.fromString(id).toString().equals(id, true)) { "Invalid job ID" }
                    val width = j.getInt("widthDots")
                    val height = j.getInt("heightDots")
                    require(width in 1..1200 && height in 1..1200) { "Invalid badge dimensions" }
                    val png = j.getString("png")
                    require(png.isNotBlank() && !png.startsWith("data:")) { "Send base64 PNG without a data prefix" }
                    Print(id.lowercase(), png, width, height)
                }
                else -> error("Unsupported kiosk message")
            }
        }
    }
}

object Zpl {
    // Composite transparent pixels against white before thresholding. MSB is the leftmost dot.
    fun graphic(width: Int, height: Int, pixels: IntArray): String {
        require(width > 0 && height > 0 && pixels.size == width * height)
        val stride = (width + 7) / 8
        val bytes = ByteArray(stride * height)
        for (y in 0 until height) for (x in 0 until width) {
            val c = pixels[y * width + x]
            val a = c ushr 24
            val luminance = (299 * (c ushr 16 and 255) + 587 * (c ushr 8 and 255) + 114 * (c and 255)) / 1000
            val visible = (luminance * a + 255 * (255 - a)) / 255
            if (visible < 128) {
                val i = y * stride + x / 8
                bytes[i] = (bytes[i].toInt() or (128 ushr (x % 8))).toByte()
            }
        }
        val digits = "0123456789ABCDEF"
        val hex = buildString(bytes.size * 2) { for (b in bytes) { append(digits[(b.toInt() and 255) ushr 4]); append(digits[b.toInt() and 15]) } }
        return "^GFA,${bytes.size},${bytes.size},$stride,$hex"
    }
    fun label(width: Int, height: Int, pixels: IntArray, dpi: Int): String {
        require(dpi == 203 || dpi == 300)
        val labelWidth = if (dpi == 203) 608 else 900
        val labelHeight = if (dpi == 203) 406 else 600
        val drawWidth = minOf(width, labelWidth)
        val drawHeight = minOf(height, labelHeight)
        val sourceX = (width - drawWidth) / 2
        val sourceY = (height - drawHeight) / 2
        val cropped = if (drawWidth == width && drawHeight == height) pixels else
            IntArray(drawWidth * drawHeight) { i -> pixels[(i / drawWidth + sourceY) * width + i % drawWidth + sourceX] }
        val x = (labelWidth - drawWidth) / 2
        val y = (labelHeight - drawHeight) / 2
        return "^XA^MNY^PW$labelWidth^LL$labelHeight^LH0,0^FO$x,$y${graphic(drawWidth, drawHeight, cropped)}^FS^PQ1^XZ"
    }
}

data class HostStatus(val ready: Boolean, val problem: String, val pending: Int = 0) {
    companion object {
        fun parse(raw: String): HostStatus {
            val frames = Regex("\u0002([^\u0003]*)\u0003").findAll(raw).map { it.groupValues[1].trim().split(',') }.toList()
            require(frames.size >= 3 && frames[0].size == 12 && frames[1].size == 11 && frames[2].size == 2) { "Printer status is incomplete" }
            val a = frames[0].map { it.trim().toIntOrNull() ?: error("Printer status is invalid") }
            val b = frames[1].map { it.trim().toIntOrNull() ?: error("Printer status is invalid") }
            val issue = when {
                a[1] != 0 -> "Printer is out of labels"
                b[2] != 0 -> "Close the printer head"
                a[2] != 0 -> "Printer is paused"
                b[3] != 0 && b[4] != 0 -> "Printer is out of ribbon"
                a[10] != 0 || a[11] != 0 -> "Printer temperature error"
                a[9] != 0 -> "Printer memory error"
                a[5] != 0 -> "Printer buffer is full"
                a[6] != 0 || a[7] != 0 -> "Printer needs attention"
                else -> ""
            }
            return HostStatus(issue.isEmpty(), issue, a[4] + b[8])
        }
    }
}
