package pub.edholm.web

import org.springframework.context.annotation.Bean
import org.springframework.core.io.ClassPathResource
import org.springframework.http.MediaType
import org.springframework.stereotype.Component
import org.springframework.web.reactive.function.server.router
import pub.edholm.db.SwoshHandler

@Component
class Router(val swoshHandler: SwoshHandler) {
    @Bean
    fun route() = router {
        accept(MediaType.TEXT_HTML).nest {
            GET("/", swoshHandler::renderIndex)
            GET("/{id}", swoshHandler::renderPreview)
            GET("/{id}/redir", swoshHandler::redirectToSwish)
        }

        // API-routes
        (accept(MediaType.APPLICATION_JSON_UTF8) and "/api").nest {
            POST("/create", swoshHandler::createSwosh)
            //GET("/preview/{id}", swoshHandler::previewSwosh)
        }
        resources("/pics/**", ClassPathResource("pics/"))
    }
}