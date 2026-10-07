package `in`.gov.kerala.bioconnect.kiosk

import android.content.ContentValues
import android.content.Context
import android.database.sqlite.SQLiteDatabase
import android.database.sqlite.SQLiteOpenHelper

// Keep all event receipts, including uncertain outcomes, so delayed UUID replays cannot reprint.
class PrintLedger(context: Context) : SQLiteOpenHelper(context, "print_receipts.db", null, 1) {
    override fun onConfigure(db: SQLiteDatabase) { db.execSQL("PRAGMA synchronous=FULL") }
    override fun onCreate(db: SQLiteDatabase) { db.execSQL("CREATE TABLE receipts (id TEXT PRIMARY KEY, response TEXT NOT NULL)") }
    override fun onUpgrade(db: SQLiteDatabase, oldVersion: Int, newVersion: Int) = Unit
    fun get(id: String): String? = readableDatabase.query("receipts", arrayOf("response"), "id = ?", arrayOf(id), null, null, null).use {
        if (it.moveToFirst()) it.getString(0) else null
    }
    fun put(id: String, response: String) {
        val row = ContentValues().apply { put("id", id); put("response", response) }
        check(writableDatabase.insertWithOnConflict("receipts", null, row, SQLiteDatabase.CONFLICT_REPLACE) != -1L) { "Could not save print receipt" }
    }
}
