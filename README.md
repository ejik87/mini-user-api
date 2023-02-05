# mini-user-api
Тестовое задание на golang




API Endpoints:

    POST /users - Create a new user
    GET /users/{id} - Get a user by ID
    PUT /users/{id} - Update a user
    DELETE /users/{id} - Delete a user
    GET /users - Get a list of users
    GET /users/search - Search for users
    POST /users/upload - Upload a list of users from an XLS\XLSX file


ENVs:
Пример переменных окружения в файле .env.example


Развёртывание приложения в docker:

```shell
docker compuse up
```