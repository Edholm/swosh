package pub.edholm

import org.springframework.boot.SpringApplication
import org.springframework.boot.autoconfigure.SpringBootApplication
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration
import org.springframework.web.reactive.config.ResourceHandlerRegistry
import org.springframework.web.reactive.config.WebFluxConfigurer
import org.springframework.web.server.ServerWebExchange
import org.springframework.web.server.WebFilter
import org.springframework.web.server.WebFilterChain
import reactor.core.publisher.Mono

@SpringBootApplication
@Configuration
class SwoshApplication : WebFluxConfigurer {
    override fun addResourceHandlers(registry: ResourceHandlerRegistry) {
        registry.addResourceHandler("/**")
                .addResourceLocations("classpath:/static/")
    }

    @Bean
    fun filter() = IndexWebFilter()
}

class IndexWebFilter : WebFilter {
    override fun filter(exchange: ServerWebExchange, chain: WebFilterChain): Mono<Void> {
        val request = exchange.request
        val path = request.uri.path
        return when (path.endsWith("/")) {
            true -> chain.filter(exchange.mutate().request { b -> b.path(path + "index.html") }.build())
            else -> chain.filter(exchange)
        }
    }
}

fun main(args: Array<String>) {
    SpringApplication.run(SwoshApplication::class.java, *args)
}

