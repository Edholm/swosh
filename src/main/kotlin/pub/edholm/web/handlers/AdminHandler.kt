package pub.edholm.web.handlers

import org.slf4j.LoggerFactory
import org.springframework.data.domain.Sort
import org.springframework.http.MediaType
import org.springframework.security.access.prepost.PreAuthorize
import org.springframework.security.core.Authentication
import org.springframework.stereotype.Component
import org.springframework.web.reactive.function.BodyExtractors
import org.springframework.web.reactive.function.server.ServerRequest
import org.springframework.web.reactive.function.server.ServerResponse
import org.springframework.web.reactive.function.server.ServerResponse.ok
import org.springframework.web.reactive.function.server.ServerResponse.seeOther
import pub.edholm.db.SwoshRepository
import pub.edholm.services.PreviewService
import reactor.core.publisher.Mono
import java.net.URI

@Component
@PreAuthorize("hasRole('ADMIN')")
class AdminHandler(private val repo: SwoshRepository, private val previewService: PreviewService) {

  private val logger = LoggerFactory.getLogger(AdminHandler::class.java)

  fun renderAdmin(req: ServerRequest): Mono<ServerResponse> {
    return repo.findAll(Sort.by(Sort.Direction.DESC, "expiresOn"))
      .flatMap { previewService.convertToPreview(it, false) }
      .collectList()
      .flatMap {
        ok().contentType(MediaType.TEXT_HTML).render(
          "admin/admin",
          mapOf(
            Pair("all", it),
            Pair("count", it.size),
            Pair("user", req.principal()),
            Pair("canUpdate", req.hasAuthority("UPDATE")),
            Pair("canDelete", req.hasAuthority("DELETE"))
          )
        )
      }
  }

  fun renderSingle(req: ServerRequest): Mono<ServerResponse> {
    return repo
      .findById(req.pathVariable("id"))
      .flatMap { previewService.convertToPreview(it, false) }
      .flatMap {
        ok().contentType(MediaType.TEXT_HTML).render(
          "admin/admin-single",
          mapOf(
            Pair("all", listOf(it)),
            Pair("user", req.principal()),
            Pair("canUpdate", req.hasAuthority("UPDATE")),
            Pair("canDelete", req.hasAuthority("DELETE"))
          )
        )
      }
  }

  @PreAuthorize("hasAuthority('UPDATE')")
  fun update(req: ServerRequest): Mono<ServerResponse> {
    return req.body(BodyExtractors.toFormData())
      .flatMap {
        val values: Map<String, String> = it.toSingleValueMap()
        val originalId = values.getValue("originalId")
        val id = values.getValue("id")

        require(!id.isBlank()) { "Tried to assign a blank id to $originalId" }

        repo.findById(originalId)
          .map { swosh ->
            swosh.copy(
              id = id,
              description = values.getValue("description"),
              amount = values.getValue("amount").toInt(),
              payee = values.getValue("payee")
            )
          }
          .flatMap { swosh ->
            println(req.headers())
            if (originalId != swosh.id) {
              logger.info("Renaming $originalId -> ${swosh.id}")
              repo.save(swosh)
                .flatMap {
                  repo.deleteById(originalId)
                }
            } else {
              logger.info("Saving $swosh after update from admin")
              repo.save(swosh)
                .then()
            }
          }
          .then(seeOther(URI.create("/admin")).build())
      }
  }

  @PreAuthorize("hasAuthority('DELETE')")
  fun delete(req: ServerRequest): Mono<ServerResponse> {
    return repo
      .deleteById(req.pathVariable("id"))
      .then(seeOther(URI.create("/admin")).build())
  }

  private fun ServerRequest.hasAuthority(authority: String): Mono<Boolean> {
    return this.principal()
      .cast(Authentication::class.java)
      .map { it.authorities.any { it.authority == authority } }
  }
}
