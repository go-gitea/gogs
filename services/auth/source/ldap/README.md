# Gitea LDAP Authentication Module

## About

This authentication module attempts to authorize and authenticate a user
against an LDAP server. It provides two methods of authentication: LDAP via
BindDN, and LDAP simple authentication.

LDAP via BindDN functions like most LDAP authentication systems. First, it
queries the LDAP server using a Bind DN and searches for the user that is
attempting to sign in. If the user is found, the module attempts to bind to the
server using the user's supplied credentials. If this succeeds, the user has
been authenticated, and his account information is retrieved and passed to the
Gogs login infrastructure.

LDAP simple authentication does not utilize a Bind DN. Instead, it binds
directly with the LDAP server using the user's supplied credentials. If the bind
succeeds and no filter rules out the user, the user is authenticated.

LDAP via BindDN is recommended for most users. By using a Bind DN, the server
can perform authorization by restricting which entries the Bind DN account can
read. Further, using a Bind DN with reduced permissions can reduce security risk
in the face of application bugs.

## Usage

To use this module, add an LDAP authentication source via the Authentications
section in the admin panel. Both the LDAP via BindDN and the simple auth LDAP
share the following fields:

* Authorization Name **(required)**
  * A name to assign to the new method of authorization.

* Host **(required)**
  * The address where the LDAP server can be reached.
  * Example: mydomain.com

* Port **(required)**
  * The port to use when connecting to the server.
  * Example: 636

* Enable TLS Encryption (optional)
  * Whether to use TLS when connecting to the LDAP server.

* Admin Filter (optional)
  * An LDAP filter specifying if a user should be given administrator
      privileges. If a user accounts passes the filter, the user will be
      privileged as an administrator.
  * Example: (objectClass=adminAccount)

* First name attribute (optional)
  * The attribute of the user's LDAP record containing the user's first name.
      This will be used to populate their account information.
  * Example: givenName

* Surname attribute (optional)
  * The attribute of the user's LDAP record containing the user's surname This
      will be used to populate their account information.
  * Example: sn

* E-mail attribute **(required)**
  * The attribute of the user's LDAP record containing the user's email
      address. This will be used to populate their account information.
  * Example: mail

**LDAP via BindDN** adds the following fields:

* Bind DN (optional)
  * The DN to bind to the LDAP server with when searching for the user. This
      may be left blank to perform an anonymous search.
  * Example: cn=Search,dc=mydomain,dc=com

* Bind Password (optional)
  * The password for the Bind DN specified above, if any. _Note: The password
      is stored in plaintext at the server. As such, ensure that your Bind DN
      has as few privileges as possible._

* User Search Base **(required)**
  * The LDAP base at which user accounts will be searched for.
  * Example: ou=Users,dc=mydomain,dc=com

* User Filter **(required)**
  * An LDAP filter declaring how to find the user record that is attempting to
      authenticate. The '%s' matching parameter will be substituted with the
      user's username.
  * Example: (&(objectClass=posixAccount)(uid=%s))

**LDAP using simple auth** adds the following fields:

* User DN **(required)**
  * A template to use as the user's DN. The `%s` matching parameter will be
      substituted with the user's username.
  * Example: cn=%s,ou=Users,dc=mydomain,dc=com
  * Example: uid=%s,ou=Users,dc=mydomain,dc=com

* User Search Base (optional)
  * The LDAP base at which user accounts will be searched for.
  * Example: ou=Users,dc=mydomain,dc=com

* User Filter **(required)**
  * An LDAP filter declaring when a user should be allowed to log in. The `%s`
      matching parameter will be substituted with the user's username.
  * Example: (&(objectClass=posixAccount)(cn=%s))
  * Example: (&(objectClass=posixAccount)(uid=%s))

**Verify group membership in LDAP** uses the following fields:

* Group Search Base (optional)
  * The LDAP DN used for groups.
  * Example: ou=group,dc=mydomain,dc=com

* Group Name Filter (optional)
  * An LDAP filter declaring how to find valid groups in the above DN.
  * Example: (|(cn=gitea_users)(cn=admins))

* User Attribute in Group (optional)
  * Which user LDAP attribute is listed in the group.
  * Example: uid

* Group Attribute for User (optional)
  * Which group LDAP attribute contains an array above user attribute names.
  * Example: memberUid
