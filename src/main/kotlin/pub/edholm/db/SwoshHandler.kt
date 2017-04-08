package pub.edholm.db

import org.springframework.http.HttpStatus
import org.springframework.http.MediaType
import org.springframework.stereotype.Component
import org.springframework.web.reactive.function.server.ServerRequest
import org.springframework.web.reactive.function.server.ServerResponse
import org.springframework.web.reactive.function.server.ServerResponse.*
import org.springframework.web.reactive.function.server.body
import pub.edholm.badRequestResponse
import pub.edholm.domain.ErrorDTO
import pub.edholm.domain.generateUri
import pub.edholm.domain.toSwishDataDTO
import reactor.core.publisher.Mono
import reactor.core.publisher.toMono
import java.net.URI
import java.time.Instant

@Component
class SwoshHandler(private val repo: SwoshRepository) {
    fun redirectToSwish(req: ServerRequest) =
            repo.findOne(req.pathVariable("id"))
                    .then { s ->
                        temporaryRedirect(s.toSwishDataDTO().generateUri())
                                .build()
                    }
                    .otherwiseIfEmpty(temporaryRedirect(URI.create("/")).build())

    fun createSwosh(req: ServerRequest): Mono<ServerResponse> {
        return req.bodyToMono(SwoshDTO::class.java)
                .then { dto ->
                    val swoshErrorDTO = validateSwoshDTO(dto)
                    when {
                        swoshErrorDTO != null -> swoshErrorDTO.badRequestResponse()
                        else ->
                            constructAndInsertNewSwosh(dto)
                                    .then { (id) ->
                                        ok()
                                                .contentType(MediaType.APPLICATION_JSON_UTF8)
                                                .body(SwoshUrlDTO(id).toMono())
                                    }
                                    .otherwise {
                                        status(HttpStatus.INTERNAL_SERVER_ERROR)
                                                .contentType(MediaType.APPLICATION_JSON_UTF8)
                                                .body(ErrorDTO(reason = "Unable to generate Swosh!").toMono())
                                    }
                    }
                }
                .otherwise { _ ->
                    ErrorDTO(reason = "Invalid input format!")
                            .badRequestResponse()
                }
    }

    private fun validateSwoshDTO(dto: SwoshDTO) =
            when {
                dto.amount == null || dto.phone == null || dto.phone.isBlank() ->
                    ErrorDTO(reason = "Missing input parameters. 'phone' and 'amount' is required")
                !dto.phone.matches(Regex("[0-9 +-]{10,15}")) ->
                    ErrorDTO(reason = "Invalid phone number. Got: ${dto.phone}")
                dto.amount < 1 ->
                    ErrorDTO(reason = "Minimum allowed amount is 1. Got ${dto.amount}")
                dto.message != null && dto.message.length > 50 ->
                    ErrorDTO(reason = "Description is too long. Max 50 chars. Got ${dto.message.length}")
                else -> null
            }

    private fun constructAndInsertNewSwosh(dto: SwoshDTO): Mono<Swosh> {
        val expireOn: Instant?
        when {
            dto.expireAfterSeconds == null || dto.expireAfterSeconds == 0L -> expireOn = null
            else -> expireOn = Instant.now().plusSeconds(dto.expireAfterSeconds)
        }
        val swosh = Swosh(
                payee = dto.phone?.trim()
                        ?.replace("-", "")
                        ?.replace(" ", "") ?: "",
                amount = dto.amount ?: 1,
                description = dto.message,
                expiresOn = expireOn)
        return repo.save(swosh)
    }

    private data class SwoshUrlDTO(val id: String) {
        fun getUrl(): String {
            return "https://swosh.me/$id"
        }
    }

    private data class SwoshDTO(
            val phone: String?,
            val amount: Int?,
            val message: String?,
            val expireAfterSeconds: Long? = Swosh.DEFAULT_EXPIRY_TIME_IN_SECONDS)
}
