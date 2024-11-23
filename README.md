**Objetivo:** Desenvolver um sistema em Go que receba um CEP, identifica a cidade e retorna o clima atual
(temperatura em graus celsius, fahrenheit e kelvin) juntamente com a cidade. Esse sistema deverá implementar
OTEL(Open Telemetry) e Zipkin.

# Aplicações

O sistema que recebe a requisição de CEP encontra-se no diretório `app1` e o sistema que busca a temperatura encontra-se
no diretório `app2`.

# Setup

Para que as checagens na WeatherApi possam funcionar, é necessário criar um arquivo `.env` na raiz do projeto com a
chave de acesso da API:

```shell
cp .env.example .env
```

O ambiente de desenvolvimento pode ser configurado com o comando abaixo:

```shell
docker-compose up
```

O `docker-compose` irá subir o Zipkin, o Jaeger, o Prometheus, o OpenTelemetry Collector e as aplicações `app1` e `app2`.

Os serviços podem ser acessados conforme as URLs abaixo:

| Serviço    | URL                    |
|------------|------------------------|
| Zipkin     | http://localhost:9411  |
| Jaeger     | http://localhost:16686 |
| Prometheus | http://localhost:9090  |
| App1       | http://localhost:8080  |
| App2       | http://localhost:8181  |

# Requisições

Para facilitar o teste das aplicações, foi criado um arquivo `requests.http` na raiz do `app1` e do `app2`.