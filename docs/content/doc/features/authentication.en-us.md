---
date: "2016-12-01T16:00:00+02:00"
title: "Authentication"
slug: "authentication"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "features"
    name: "Authentication"
    weight: 10
    identifier: "authentication"
---

# Authentication

{{< toc >}}

## LDAP (Lightweight Directory Access Protocol)

Both the LDAP via BindDN and the simple auth LDAP share the following fields:

- Authorization Name **(required)**

  - A name to assign to the new method of authorization.

- Host **(required)**

  - The address where the LDAP server can be reached.
  - Example: `mydomain.com`

- Port **(required)**

  - The port to use when connecting to the server.
  - Example: `389` for LDAP or `636` for LDAP SSL

- Enable TLS Encryption (optional)

  - Whether to use TLS when connecting to the LDAP server.

- Admin Filter (optional)

  - An LDAP filter specifying if a user should be given administrator
    privileges. If a user account passes the filter, the user will be
    privileged as an administrator.
  - Example: `(objectClass=adminAccount)`
  - Example for Microsoft Active Directory (AD): `(memberOf=CN=admin-group,OU=example,DC=example,DC=org)`

- Username attribute (optional)

  - The attribute of the user's LDAP record containing the user name. Given
    attribute value will be used for new Gitea account user name after first
    successful sign-in. Leave empty to use login name given on sign-in form.
  - This is useful when supplied login name is matched against multiple
    attributes, but only single specific attribute should be used for Gitea
    account name, see "User Filter".
  - Example: `uid`
  - Example for Microsoft Active Directory (AD): `sAMAccountName`

- First name attribute (optional)

  - The attribute of the user's LDAP record containing the user's first name.
    This will be used to populate their account information.
  - Example: `givenName`

- Surname attribute (optional)

  - The attribute of the user's LDAP record containing the user's surname.
    This will be used to populate their account information.
  - Example: `sn`

- E-mail attribute **(required)**
  - The attribute of the user's LDAP record containing the user's email
    address. This will be used to populate their account information.
  - Example: `mail`

### LDAP via BindDN

Adds the following fields:

- Bind DN (optional)

  - The DN to bind to the LDAP server with when searching for the user. This
    may be left blank to perform an anonymous search.
  - Example: `cn=Search,dc=mydomain,dc=com`

- Bind Password (optional)

  - The password for the Bind DN specified above, if any. _Note: The password
    is stored encrypted with the SECRET_KEY on the server. It is still recommended
    to ensure that the Bind DN has as few privileges as possible._

- User Search Base **(required)**

  - The LDAP base at which user accounts will be searched for.
  - Example: `ou=Users,dc=mydomain,dc=com`

- User Filter **(required)**
  - An LDAP filter declaring how to find the user record that is attempting to
    authenticate. The `%s` matching parameter will be substituted with login
    name given on sign-in form.
  - Example: `(&(objectClass=posixAccount)(uid=%s))`
  - Example for Microsoft Active Directory (AD): `(&(objectCategory=Person)(memberOf=CN=user-group,OU=example,DC=example,DC=org)(sAMAccountName=%s)(!(UserAccountControl:1.2.840.113556.1.4.803:=2)))`
  - To substitute more than once, `%[1]s` should be used instead, e.g. when
    matching supplied login name against multiple attributes such as user
    identifier, email or even phone number.
  - Example: `(&(objectClass=Person)(|(uid=%[1]s)(mail=%[1]s)(mobile=%[1]s)))`
- Enable user synchronization
  - This option enables a periodic task that synchronizes the Gitea users with
    the LDAP server. The default period is every 24 hours but that can be
    changed in the app.ini file. See the _cron.sync_external_users_ section in
    the [sample
    app.ini](https://github.com/go-gitea/gitea/blob/master/custom/conf/app.example.ini)
    for detailed comments about that section. The _User Search Base_ and _User
    Filter_ settings described above will limit which users can use Gitea and
    which users will be synchronized. When initially run the task will create
    all LDAP users that match the given settings so take care if working with
    large Enterprise LDAP directories.

### LDAP using simple auth

Adds the following fields:

- User DN **(required)**

  - A template to use as the user's DN. The `%s` matching parameter will be
    substituted with login name given on sign-in form.
  - Example: `cn=%s,ou=Users,dc=mydomain,dc=com`
  - Example: `uid=%s,ou=Users,dc=mydomain,dc=com`

- User Search Base (optional)

  - The LDAP base at which user accounts will be searched for.
  - Example: `ou=Users,dc=mydomain,dc=com`

- User Filter **(required)**
  - An LDAP filter declaring when a user should be allowed to log in. The `%s`
    matching parameter will be substituted with login name given on sign-in
    form.
  - Example: `(&(objectClass=posixAccount)(cn=%s))`
  - Example: `(&(objectClass=posixAccount)(uid=%s))`

### Verify group membership in LDAP

Uses the following fields:

- Group Search Base (optional)

  - The LDAP DN used for groups.
  - Example: `ou=group,dc=mydomain,dc=com`

- Group Name Filter (optional)

  - An LDAP filter declaring how to find valid groups in the above DN.
  - Example: `(|(cn=gitea_users)(cn=admins))`

- User Attribute in Group (optional)

  - Which user LDAP attribute is listed in the group.
  - Example: `uid`

- Group Attribute for User (optional)
  - Which group LDAP attribute contains an array above user attribute names.
  - Example: `memberUid`

## PAM (Pluggable Authentication Module)

To configure PAM, set the 'PAM Service Name' to a filename in `/etc/pam.d/`. To
work with normal Linux passwords, the user running Gitea must have read access
to `/etc/shadow`.

## SMTP (Simple Mail Transfer Protocol)

This option allows Gitea to log in to an SMTP host as a Gitea user. To
configure this, set the fields below:

- Authentication Name **(required)**

  - A name to assign to the new method of authorization.

- SMTP Authentication Type **(required)**

  - Type of authentication to use to connect to SMTP host, PLAIN or LOGIN.

- Host **(required)**

  - The address where the SMTP host can be reached.
  - Example: `smtp.mydomain.com`

- Port **(required)**

  - The port to use when connecting to the server.
  - Example: `587`

- Allowed Domains

  - Restrict what domains can log in if using a public SMTP host or SMTP host
    with multiple domains.
  - Example: `gitea.io,mydomain.com,mydomain2.com`

- Enable TLS Encryption

  - Enable TLS encryption on authentication.

- Skip TLS Verify

  - Disable TLS verify on authentication.

- This authentication is activate
  - Enable or disable this auth.

## FreeIPA

- In order to log in to Gitea using FreeIPA credentials, a bind account needs to
  be created for Gitea:

- On the FreeIPA server, create a `gitea.ldif` file, replacing `dc=example,dc=com`
  with your DN, and provide an appropriately secure password:

  ```sh
  dn: uid=gitea,cn=sysaccounts,cn=etc,dc=example,dc=com
  changetype: add
  objectclass: account
  objectclass: simplesecurityobject
  uid: gitea
  userPassword: secure password
  passwordExpirationTime: 20380119031407Z
  nsIdleTimeout: 0
  ```

- Import the LDIF (change localhost to an IPA server if needed). A prompt for
  Directory Manager password will be presented:

  ```sh
  ldapmodify -h localhost -p 389 -x -D \
  "cn=Directory Manager" -W -f gitea.ldif
  ```

- Add an IPA group for gitea_users :

  ```sh
  ipa group-add --desc="Gitea Users" gitea_users
  ```

- Note: For errors about IPA credentials, run `kinit admin` and provide the
  domain admin account password.

- Log in to Gitea as an Administrator and click on "Authentication" under Admin Panel.
  Then click `Add New Source` and fill in the details, changing all where appropriate.

## SPNEGO with SSPI (Kerberos/NTLM, for Windows only)

Gitea supports SPNEGO single sign-on authentication (the scheme defined by RFC4559) for the web part of the server via the Security Support Provider Interface (SSPI) built in Windows. SSPI works only in Windows environments - when both the server and the clients are running Windows.

Before activating SSPI single sign-on authentication (SSO) you have to prepare your environment:

- Create a separate user account in active directory, under which the `gitea.exe` process will be running (eg. `user` under domain `domain.local`):

- Create a service principal name for the host where `gitea.exe` is running with class `HTTP`:

  - Start `Command Prompt` or `PowerShell` as a privileged domain user (eg. Domain Administrator)
  - Run the command below, replacing `host.domain.local` with the fully qualified domain name (FQDN) of the server where the web application will be running, and `domain\user` with the name of the account created in the previous step:

  ```sh
  setspn -A HTTP/host.domain.local domain\user
  ```

- Sign in (_sign out if you were already signed in_) with the user created

- Make sure that `ROOT_URL` in the `[server]` section of `custom/conf/app.ini` is the fully qualified domain name of the server where the web application will be running - the same you used when creating the service principal name (eg. `host.domain.local`)

- Start the web server (`gitea.exe web`)

- Enable SSPI authentication by adding an `SPNEGO with SSPI` authentication source in `Site Administration -> Authentication Sources`

- Sign in to a client computer in the same domain with any domain user (client computer, different from the server running `gitea.exe`)

- If you are using Chrome or Edge, add the URL of the web app to the Local intranet sites (`Internet Options -> Security -> Local intranet -> Sites`)

- Start Chrome or Edge and navigate to the FQDN URL of gitea (eg. `http://host.domain.local:3000`)

- Click the `Sign In` button on the dashboard and choose SSPI to be automatically logged in with the same user that is currently logged on to the computer

- If it does not work, make sure that:
  - You are not running the web browser on the same server where gitea is running. You should be running the web browser on a domain joined computer (client) that is different from the server. If both the client and server are running on the same computer NTLM will be preferred over Kerberos.
  - There is only one `HTTP/...` SPN for the host
  - The SPN contains only the hostname, without the port
  - You have added the URL of the web app to the `Local intranet zone`
  - The clocks of the server and client should not differ with more than 5 minutes (depends on group policy)
  - `Integrated Windows Authentication` should be enabled in Internet Explorer (under `Advanced settings`)
