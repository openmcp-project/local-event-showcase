FROM node:24.20@sha256:be23f54a88d34e8824c741b19b91064094f92c1c97b194144bfc8b50d67258e2 AS build

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
