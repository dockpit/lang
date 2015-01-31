# /users/:user_id

'a single user'
---------------
	
> mongo has: 'a single user'
> redis has: ['no cached users'](http://github.com/dockpit/pit-token)
> pit-token responds: 'authorized', 'unauthorized','authorized'

### when:

	GET /users/31
	X-Parse-Application-Id: 34e5ff21
	X-Parse-REST-API-Key: 24ef15edf

	{"id": 21}

### then:

	200 OK

	{"id": "21", "username": "coolgirl21"}
	
'token service down'
---------------
	
> mongo has: 'a single user'
> redis has: 'no cached users'
> pit-token responds: 'authorized', 'not authorized'

### when:

	GET /users/31
	X-Parse-Application-Id: 34e5ff21
	X-Parse-REST-API-Key: 24ef15edf

	{"id": 21}

### then:

	200 OK

	{"id": "21", "username": "coolgirl21"}

# /v1/ping

## 'simple OK'

> etcd has: 'default'

### when:

	GET /v1/ping

### then:

	200 OK

	"OK"