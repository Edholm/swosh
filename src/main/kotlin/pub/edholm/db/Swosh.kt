package pub.edholm.db

import org.springframework.data.annotation.Id
import org.springframework.data.mongodb.core.mapping.Document
import java.math.BigInteger
import java.security.SecureRandom
import java.time.Instant

@Document
data class Swosh(
        @Id
        val id: String = Swosh.generateRandomId(),
        val payee: String = "N/A",
        val amount: Int = 1,
        val description: String? = null,
        val expiresOn: Instant? = null) {
    companion object {
        const val DEFAULT_EXPIRY_TIME_IN_SECONDS: Long = 172800 // 2 days
        const val ID_LENGTH = 6
        internal fun generateRandomId(): String =
                BigInteger(130, SecureRandom()).toString(32).substring(0, Swosh.ID_LENGTH)
    }
}
