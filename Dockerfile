FROM node:lts-alpine as frontend

WORKDIR /frontend-build

COPY web/package.json web/yarn.lock ./
RUN yarn install

COPY web ./
RUN yarn build

FROM golang:1.20.5-alpine as backend

RUN apk add --no-cache gcc musl-dev linux-headers

WORKDIR /backend-build

COPY go.* ./
RUN go mod download

COPY . .
COPY --from=frontend /frontend-build/dist web/dist

RUN go build -o eth-faucet -ldflags "-s -w"

FROM alpine

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=backend /backend-build/eth-faucet ./eth-faucet

ENTRYPOINT ["./eth-faucet"]