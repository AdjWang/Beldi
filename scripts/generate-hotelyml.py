# -*- coding: utf-8 -*-

template = """\
  beldi-dev-{0}:
    lang: go
    handler: ./cmd/handler/hotel/{0}
    image: localhost:5001/{0}:latest
    build_args:
      GO111MODULE: on
      GOPROXY: https://goproxy.cn
"""

# print(template.format("geo"))

def generate_function_configs(fn_names):
    config = []
    for name in fn_names:
        config.append(template.format(name))
    return "".join(config)

print(generate_function_configs([
    "geo",
    "profile",
    "rate",
    "recommendation",
    "user",
    "hotel",
    "search",
    "flight",
    "order",
    "frontend",
    "gateway",
    "hotelgc",
    "hotelcollector",
]))
