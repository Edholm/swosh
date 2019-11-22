package pub.edholm

import com.samskivert.mustache.Mustache
import io.micrometer.core.instrument.MeterRegistry
import org.springframework.boot.ApplicationRunner
import org.springframework.boot.actuate.autoconfigure.metrics.MeterRegistryCustomizer
import org.springframework.boot.autoconfigure.SpringBootApplication
import org.springframework.boot.autoconfigure.mustache.MustacheProperties
import org.springframework.boot.autoconfigure.mustache.MustacheResourceTemplateLoader
import org.springframework.boot.context.properties.EnableConfigurationProperties
import org.springframework.boot.runApplication
import org.springframework.boot.web.reactive.result.view.MustacheViewResolver
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration
import org.springframework.data.mongodb.config.EnableMongoAuditing
import org.springframework.scheduling.annotation.EnableScheduling
import org.springframework.security.core.authority.AuthorityUtils
import org.springframework.security.crypto.factory.PasswordEncoderFactories
import org.springframework.web.reactive.config.WebFluxConfigurer
import pub.edholm.db.User
import pub.edholm.db.UserRepository
import java.net.InetAddress

@SpringBootApplication
@EnableConfigurationProperties(Properties::class)
@EnableScheduling
@EnableMongoAuditing
@Configuration
class SwoshApplication(private val props: MustacheProperties) : WebFluxConfigurer {

  private val mustacheCompiler = Mustache
    .compiler()
    .escapeHTML(true)
    .withLoader(MustacheResourceTemplateLoader(props.prefix, props.suffix))

  @Bean
  fun viewResolver() = MustacheViewResolver(mustacheCompiler).apply {
    setPrefix(props.prefix)
    setSuffix(props.suffix)
  }

  @Bean
  fun createAdminUsers(userRepository: UserRepository, properties: Properties) = ApplicationRunner {
    val pwdEncoder = PasswordEncoderFactories.createDelegatingPasswordEncoder()
    val users = properties.users
      .filterNot { userRepository.existsByUsername(it.username).block() ?: false }
      .map {
        User(
          username = it.username,
          password = pwdEncoder.encode(it.password),
          authorities = AuthorityUtils.createAuthorityList("ROLE_USER", "ROLE_ADMIN", "DELETE", "UPDATE")
        )
      }
    userRepository.saveAll(users).subscribe()
  }

  @Bean
  fun metricsCommonTags(properties: Properties): MeterRegistryCustomizer<MeterRegistry> {
    val hostName = properties.metrics.serverHostname.ifBlank { InetAddress.getLocalHost().hostName }
    return MeterRegistryCustomizer {
      it.config()
        .commonTags(
          "env", properties.metrics.environment,
          "host", hostName,
          "app", properties.metrics.appName
        )
    }
  }
}

fun main(args: Array<String>) {
  runApplication<SwoshApplication>(*args)
}

