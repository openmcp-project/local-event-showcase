FROM node:26.12@sha256:97fa0f1b59035c6072721a88d55be1371c7cb1cc942b521e1cc5c72ec87655f4 AS build

COPY ./ /app

WORKDIR /app
RUN npm ci

RUN npm run build

FROM nginx:alpine

COPY --from=build /app/dist-wc /usr/share/nginx/html/ui/openmcp/wc
COPY nginx.conf /etc/nginx/nginx.conf

# Set permissions for nginx user (already exists in nginx:alpine)
RUN chown -R nginx:nginx /var/cache/nginx /var/run /var/log/nginx /usr/share/nginx/html \
    && chmod -R 755 /usr/share/nginx/html \
    && touch /var/run/nginx.pid \
    && chown nginx:nginx /var/run/nginx.pid

USER nginx

EXPOSE 8080

CMD ["nginx", "-g", "daemon off;"]
