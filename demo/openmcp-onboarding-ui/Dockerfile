FROM node:24.21@sha256:3d27e5c11e5786e309ec3e03f93ae536eb36e6e5eb3714d5eb3300a36157add0 AS build

COPY ./ /app

WORKDIR /app
RUN npm ci

RUN npm run build

FROM nginx:alpine@sha256:a3554c0d0b48177b1413c26e01f695334428020d9df01243fbae3e11cfd17417

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
