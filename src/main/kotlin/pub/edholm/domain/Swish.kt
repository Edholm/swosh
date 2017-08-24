package pub.edholm.domain

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.google.common.net.UrlEscapers
import pub.edholm.db.Swosh
import java.net.URI


data class StringValue(
        val value: String,
        val editable: Boolean = false
)

data class IntValue(
        val value: Int,
        val editable: Boolean = false
)

data class SwishDataDTO(
        val version: Int = 1,
        val payee: StringValue,
        val amount: IntValue,
        val message: StringValue
)

fun SwishDataDTO.generateUri(): URI {
    val asString = jacksonObjectMapper().writeValueAsString(this)
    val encodedData = UrlEscapers.urlFragmentEscaper().escape(asString)
    return URI.create("swish://payment?data=$encodedData")
}

/**
 * Generates the string requred by the QR scanner built into swish.
 *
 * Swish uses multiple formats for input:
 * A[PHONENUMBER]
 * B[PHONENUMBER];[AMOUNT]
 * C[PHONENUMBER];[AMOUNT];[MESSAGE];[LOCK_INPUT_FIELDS_CONFIGURATION]
 *
 * [PHONENUMBER] is a string containing the receiver phone number. Country code, dashes and spaces are optional.
 * [AMOUNT] is a number, integer or double, with the amount to send.
 * [MESSAGE] is a message to include to the receiver.
 * [LOCK_INPUT_FIELDS_CONFIGURATION] describes which fields should be editable after autofill.
 * 0: none
 * 1: phonenumber
 * 2: amount
 * 4: message
 * These can be combined in a bitwise manner. E.g: 1 + 4 = 5 for phonenumber and message field locked
 *
 */
fun SwishDataDTO.generateSwishQRString(): String {
    var lockFields = 0
    if (this.payee.editable)lockFields += 1
    if (this.amount.editable)lockFields += 2
    if (this.message.editable)lockFields += 4
    return "C${this.payee.value};${this.amount.value};${this.message.value};${lockFields}"
}

fun Swosh.toSwishDataDTO(): SwishDataDTO {
    return SwishDataDTO(
            payee = StringValue(this.payee, false),
            amount = IntValue(this.amount, false),
            message = StringValue(this.description ?: "", true))
}
