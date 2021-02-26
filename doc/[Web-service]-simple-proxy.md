## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server), the simple web proxy offers
visitor access to websites internal to laitos host.

The proxy is not designed to provide anonymity.

## Configuration
Under JSON key `HTTPHandlers`, write a string property called `WebProxyEndpoint`, value being the URL location that will
serve the web proxy. Keep the location a secret to yourself and make it difficult to guess.

Here is an example:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "WebProxyEndpoint": "/very-secret-web-proxy",

        ...
    },

    ...
}
</pre>

## Run
The form is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage
In a web browser, navigate to `WebProxyEndpoint` of laitos web server. At end of the URL, append an HTTP parameter `u`
so that the entire URL looks like:

    https://my-laitos-server.net/very-secret-web-proxy?u=<ENCODED URL>

For example, to visit `github.com`:

    https://my-laitos-server.net/very-secret-web-proxy?u=https%3A%2F%2Fgithub.com

Several seconds after the page loads, two buttons `XY` and `XY-ALL` will appear near each corner:
- `XY` button prepares image, link, and form submission URLs for proxy operation.
- In addition to `XY`'s items, `XY-ALL` button prepares iframe and script URLs for proxy operation. This may cause page
  to lose information that you have already entered.

Click on `XY` or `XY-ALL` button as required, to continue browsing. The buttons will stay on the page.

## Tips
- Make the endpoint difficult to guess, this helps to prevent misuse of the service.
- The web proxy does not provide anonymity, and it may fail to properly render sophisticated web pages.
- Also consider using the [desktop on-a-page (virtual machine)](https://github.com/HouzuoGuo/laitos/wiki/%5BWeb-service%5D-desktop-on-a-page-(virtual-machine))
  to launch a remotely controlled virtual machine and web browser, which is better at rendering sophisticated web pages.
