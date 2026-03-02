FROM node:22-alpine AS build

WORKDIR /app

COPY docs/package.json docs/package-lock.json* ./docs/
RUN cd docs && if [ -f package-lock.json ]; then npm ci; else npm install; fi

COPY docs/ ./docs/

RUN cd docs && npm run build

FROM nginx:alpine

COPY --from=build /app/docs/dist /usr/share/nginx/html/fuku

RUN printf 'server {\n\
    listen 80;\n\
    server_name localhost;\n\
    location /fuku/ {\n\
        alias /usr/share/nginx/html/fuku/;\n\
        try_files $uri $uri/ =404;\n\
    }\n\
    location = / {\n\
        return 302 /fuku/;\n\
    }\n\
}\n' > /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
