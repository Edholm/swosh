package pub.edholm.domain

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import pub.edholm.db.Swosh
import java.net.URI
import java.net.URLEncoder
import java.nio.charset.StandardCharsets


data class StringValue(
        val value: String
)

data class IntValue(
        val value: Int
)

data class SwishDataDTO(
        val version: Int = 1,
        val payee: StringValue,
        val amount: IntValue,
        val message: StringValue
)

fun SwishDataDTO.generateUri(): URI {
    val asString = jacksonObjectMapper().writeValueAsString(this)
    val encodedData = URLEncoder.encode(asString, StandardCharsets.UTF_8.displayName())
    return URI.create("swish://payment?data=$encodedData")
}

fun Swosh.toSwishDataDTO(): SwishDataDTO {
    return SwishDataDTO(
            payee = StringValue(this.payee),
            amount = IntValue(this.amount),
            message = StringValue(this.description ?: ""))
}
