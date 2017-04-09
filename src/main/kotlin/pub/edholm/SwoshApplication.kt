package pub.edholm

import com.samskivert.mustache.Mustache
import org.springframework.boot.SpringApplication
import org.springframework.boot.autoconfigure.SpringBootApplication
import org.springframework.boot.autoconfigure.mustache.MustacheProperties
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration
import org.springframework.scheduling.annotation.EnableScheduling
import org.springframework.web.reactive.config.WebFluxConfigurer
import pub.edholm.support.MustacheResourceTemplateLoader
import pub.edholm.support.MustacheViewResolver


@SpringBootApplication
@EnableScheduling
@Configuration
class SwoshApplication : WebFluxConfigurer {
    @Bean
    fun mustacheViewResolver(props: MustacheProperties): MustacheViewResolver {
        val viewResolver = MustacheViewResolver()
        viewResolver.setPrefix(props.prefix)
        viewResolver.setSuffix(props.suffix)
        val loader = MustacheResourceTemplateLoader(props.prefix, props.suffix)
        viewResolver.setCompiler(Mustache.compiler().escapeHTML(false).withLoader(loader))
        return viewResolver
    }
}

fun main(args: Array<String>) {
    SpringApplication.run(SwoshApplication::class.java, *args)
}

