package pub.edholm.db

import org.springframework.data.annotation.Id
import org.springframework.data.mongodb.core.mapping.Document
import java.math.BigInteger
import java.security.SecureRandom
import java.time.Instant

@Document
data class Swosh(
        @Id
        val id: String = BigInteger(130, SecureRandom()).toString(32).substring(0, Swosh.ID_LENGTH),
        val payee: String = "N/A",
        val amount: Int = 1,
        val description: String? = null,
        val expiresOn: Instant? = Instant.now().plusSeconds(Swosh.DEFAULT_EXPIRY_TIME_IN_SECONDS)) {
    companion object {
        const val DEFAULT_EXPIRY_TIME_IN_SECONDS: Long = 172800 // 2 days
        const val ID_LENGTH = 6
    }
}
