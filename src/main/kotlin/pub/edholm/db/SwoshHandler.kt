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
class SwoshHandler(val repo: SwoshRepository) {
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
                    when {
                        dto.amount == null || dto.phone.isNullOrBlank() ->
                            ErrorDTO(reason = "Missing input parameters. 'phone' and 'amount' is required")
                                    .badRequestResponse()
                        dto.amount < 1 ->
                            ErrorDTO(reason = "Minimum allowed amount is 1. Got ${dto.amount}")
                                    .badRequestResponse()
                        dto.description != null && dto.description.length > 50 ->
                            ErrorDTO(reason = "Description is too long. Max 50 chars. Got ${dto.description.length}")
                                    .badRequestResponse()
                        else -> constructAndInsertNewSwosh(dto)
                                .then { s ->
                                    ok()
                                            .contentType(MediaType.APPLICATION_JSON_UTF8)
                                            .body(SwoshUrlDTO(s.id).toMono())
                                }
                                .otherwise {
                                    status(HttpStatus.INTERNAL_SERVER_ERROR)
                                            .contentType(MediaType.APPLICATION_JSON_UTF8)
                                            .body(ErrorDTO(reason = "Unable to generate Swosh!").toMono())
                                }
                    }
                }
                .otherwise { e ->
                    ErrorDTO(reason = "Invalid input format!")
                            .badRequestResponse()
                }
    }

    private fun constructAndInsertNewSwosh(dto: SwoshDTO): Mono<Swosh> {
        val swosh = Swosh(payee = dto.phone ?: "",
                amount = dto.amount ?: 1,
                description = dto.description,
                expiresOn = Instant.now().plusSeconds(dto.expireAfterSeconds ?: Swosh.DEFAULT_EXPIRY_TIME_IN_SECONDS))
        return repo.save(swosh)
    }

    private data class SwoshUrlDTO(val url: String)

    private data class SwoshDTO(
            val phone: String?,
            val amount: Int?,
            val description: String?,
            val expireAfterSeconds: Long? = Swosh.DEFAULT_EXPIRY_TIME_IN_SECONDS)
}
