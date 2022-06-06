openssl genrsa -out testbin/webserverkey.pem 2048
openssl req -new -x509 -sha256 -key testbin/webserverkey.pem -out testbin/webservercert.pem -days 3650 -subj '/CN=www.contoso.com/O=Contoso LTD./C=US'
sudo bats integration-test/test/test.bats