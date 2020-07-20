# RBAC
alpha feature. now the feature is very simple in early stage. only has root account authentication

you can choose to enable RBAC feature, after enable RBAC, all request to service center must be authenticated

### Configuration file
follow steps to enable this feature.

1.get rsa key pairs
```sh
openssl genrsa -out private.key 4096
openssl rsa -in private.key -pubout -out public.key
```

2.edit app.conf
```ini
rbac_enabled = true
rbac_rsa_public_key_file = ./public.key # rsa key pairs
rbac_rsa_private_key_file = ./private.key # rsa key pairs
auth_plugin = buildin # must set to buildin
```
3.root account

before you start server, you need to set env to set your root account password.  

```sh
export SC_INIT_ROOT_PASSWORD=rootpwd
```
at the first time service center cluster init, it will use this password to setup rbac module. 
you can revoke password by rest API after cluster started. but you can not use this env to revoke password after cluster started.

the root account name is "root"

To securely distribute your root account and private key, 
you can use kubernetes [secret](https://kubernetes.io/zh/docs/tasks/inject-data-application/distribute-credentials-secure/)
### Generate a token 
token is the only credential to access rest API, before you access any API, you need to get a token
```shell script
curl -X POST \
  http://127.0.0.1:30100/v4/token \
  -d '{"name":"root",
"password":"rootpwd"}'
```
will return a token, token will expired after 30m
```json
{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1OTI4MzIxODUsInVzZXIiOiJyb290In0.G65mgb4eQ9hmCAuftVeVogN9lT_jNg7iIOF_EAyAhBU"}
```

### Authentication
in each request you must add token to  http header:
```
Authorization: Bearer {token}
```
for example:
```shell script
curl -X GET \
  'http://127.0.0.1:30100/v4/default/registry/microservices/{service-id}/instances' \
  -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1OTI4OTQ1NTEsInVzZXIiOiJyb290In0.FfLOSvVmHT9qCZSe_6iPf4gNjbXLwCrkXxKHsdJoQ8w' 
```

### Change password
You must supply current password and token to update to new password
```shell script
curl -X PUT \
  http://127.0.0.1:30100/v4/reset-password \
  -H 'Authorization: Bearer {your_token}' \
  -d '{
	"currentPassword":"rootpwd",
	"password":"123"
}'
```

### create a new account by account which has admin role 
```shell script
curl -X POST \
  http://127.0.0.1:30100/v4/account \
  -H 'Accept: */*' \
  -H 'Authorization: Bearer {your_token}' \
  -H 'Content-Type: application/json' \
  -d '{
	"name":"peter",
	"password":"{strong_password}",
	"role":"developer"
	
}'
```
### Roles 
currently, you can not custom and manage any role and role policy. there is only 2 build in roles. rbac feature is in early development stage.
- admin: able to do anything, including manage account, even change other account password
- developer: able to call most of API except account management. except account management