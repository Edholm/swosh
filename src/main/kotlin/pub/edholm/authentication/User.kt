package pub.edholm.authentication

import org.springframework.data.annotation.Id
import org.springframework.data.mongodb.core.index.Indexed
import org.springframework.data.mongodb.core.mapping.Document
import org.springframework.security.core.GrantedAuthority
import org.springframework.security.core.authority.AuthorityUtils
import org.springframework.security.core.userdetails.UserDetails
import java.util.*

@Document
data class User(
  @Id
  val id: UUID = UUID.randomUUID(),
  @Indexed(unique = true, sparse = true)
  private val username: String,
  private val password: String,
  private val isEnabled: Boolean = true,
  private val authorities: Collection<GrantedAuthority> = AuthorityUtils.createAuthorityList("USER")
) : UserDetails {
  override fun getUsername(): String = username
  override fun getPassword(): String = password
  override fun isEnabled(): Boolean = isEnabled
  override fun isCredentialsNonExpired(): Boolean = true
  override fun isAccountNonExpired(): Boolean = true
  override fun isAccountNonLocked(): Boolean = true
  override fun getAuthorities(): Collection<GrantedAuthority> = authorities
}



