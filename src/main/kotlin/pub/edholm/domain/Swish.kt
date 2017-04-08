package pub.edholm.domain

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import org.springframework.web.util.UriUtils
import pub.edholm.db.Swosh
import java.net.URI
import java.nio.charset.StandardCharsets


data class Value(
        val value: String
)

data class SwishDataDTO(
        val version: Int = 1,
        val payee: Value,
        val amount: Value,
        val message: Value
)

fun SwishDataDTO.generateUri(): URI {
    val mapper = jacksonObjectMapper()
    val dtoAsString = UriUtils.encode(mapper.writeValueAsString(this), StandardCharsets.UTF_8)
    return URI.create("swish://payment?data=$dtoAsString")
}

fun Swosh.toSwishDataDTO(): SwishDataDTO {
    return SwishDataDTO(
            payee = Value(this.payee),
            amount = Value(this.amount.toString()),
            message = Value(this.description ?: ""))
}
