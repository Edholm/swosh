package pub.edholm

import org.springframework.boot.context.properties.ConfigurationProperties
import org.springframework.boot.context.properties.ConstructorBinding

@ConstructorBinding
@ConfigurationProperties("swosh")
data class Properties(
  val hostname: String,
  val scheme: String,
  val provisionUsers: Boolean,
  val users: List<User>,
  val metrics: Metrics
) {
  data class User(val username: String, val password: String)
  data class Metrics(val serverHostname: String, val environment: String, val appName: String)
}
