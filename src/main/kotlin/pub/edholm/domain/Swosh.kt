package pub.edholm.domain

import com.google.i18n.phonenumbers.PhoneNumberUtil
import pub.edholm.db.Swosh
import java.time.Instant

data class SwoshUrlDTO(val id: String, private val hostname: String = "swosh.me", private val scheme: String = "https") {
  fun getUrl(): String {
    return "$scheme://$hostname/$id"
  }
}

data class SwoshDTO(
  val phone: String?,
  val amount: Int?,
  val message: String?,
  val expireAfterSeconds: Long? = Swosh.DEFAULT_EXPIRY_TIME_IN_SECONDS
)

fun SwoshDTO.toSwosh(): Swosh {
  val expireOn: Instant? = when {
    this.expireAfterSeconds == null || this.expireAfterSeconds == 0L -> null
    else -> Instant.now().plusSeconds(this.expireAfterSeconds)
  }
  val phoneUtil = PhoneNumberUtil.getInstance()
  val parsedNumber = phoneUtil.parse(this.phone, "SE")
  val formattedNumber = phoneUtil.format(parsedNumber, PhoneNumberUtil.PhoneNumberFormat.NATIONAL)
  return Swosh(
    payee = formattedNumber,
    amount = this.amount ?: 1,
    description = if (message.isNullOrBlank()) null else this.message,
    expiresOn = expireOn
  )

}

