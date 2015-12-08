FROM nginx
COPY sources.list /etc/apt/sources.list
RUN apt-get update && \
    apt-get install -y curl && \
    apt-get install -y vim && \
    apt-get install -y dnsutils

COPY controller /
COPY default.conf /etc/nginx/nginx.conf

EXPOSE 80 443
CMD ["/controller"]