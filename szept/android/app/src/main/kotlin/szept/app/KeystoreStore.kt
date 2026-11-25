package szept.app

import android.content.Context
import android.content.pm.PackageManager
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.security.keystore.StrongBoxUnavailableException
import android.util.AtomicFile
import szept.client.StateStore
import java.io.File
import java.io.FileNotFoundException
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * The app state in one file, encrypted with an AES-256-GCM key that never
 * leaves the Android Keystore (StrongBox when the phone has one) (§2).
 * File layout: 12-byte IV || ciphertext+tag.
 */
class KeystoreStore(private val ctx: Context) : StateStore {
    private val file = AtomicFile(File(ctx.filesDir, "state.bin"))

    private fun key(): SecretKey {
        val ks = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (ks.getKey(ALIAS, null) as SecretKey?)?.let { return it }
        val strongBox = ctx.packageManager.hasSystemFeature(PackageManager.FEATURE_STRONGBOX_KEYSTORE)
        return try {
            generate(strongBox)
        } catch (e: StrongBoxUnavailableException) {
            generate(false)
        }
    }

    private fun generate(strongBox: Boolean): SecretKey {
        val spec = KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
            .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
            .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
            .setKeySize(256)
            .setIsStrongBoxBacked(strongBox)
            .build()
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").run {
            init(spec)
            generateKey()
        }
    }

    override fun load(): ByteArray? {
        val data = try {
            file.readFully()
        } catch (e: FileNotFoundException) {
            return null
        }
        val c = Cipher.getInstance(TRANSFORM)
        c.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, data, 0, 12))
        return c.doFinal(data, 12, data.size - 12)
    }

    override fun save(data: ByteArray) {
        val c = Cipher.getInstance(TRANSFORM)
        c.init(Cipher.ENCRYPT_MODE, key()) // the Keystore picks the IV
        val out = c.iv + c.doFinal(data)
        val s = file.startWrite()
        try {
            s.write(out)
            file.finishWrite(s)
        } catch (e: Exception) {
            file.failWrite(s)
            throw e
        }
    }

    private companion object {
        const val ALIAS = "szept-state"
        const val TRANSFORM = "AES/GCM/NoPadding"
    }
}
