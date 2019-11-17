package pub.edholm

import org.springframework.boot.context.properties.ConfigurationProperties
import org.springframework.boot.context.properties.ConstructorBinding

@ConstructorBinding
@ConfigurationProperties("swosh")
data class Properties(
  val hostname: String,
  val scheme: String,
  val users: List<User>
) {
  data class User(val username: String, val password: String)
}
