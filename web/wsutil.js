let wsSock = null
let wsConnected = false;
function useWebsocket(handlers) {
	let url = location.origin.replace("http", "ws") + "/api/ws";
	console.log("Creating WebSocket to", url);
	wsSock = new WebSocket(url);
	wsSock.onclose = _ => {
		wsConnected = false;
		setTimeout(useWebsocket.bind(null, handlers), 1000);
	};
	wsSock.onerror = evt => {
		console.error("Connection errored:", evt);
	};
	wsSock.onopen = _ => {
		console.log("WS connection opened");
		wsConnected = true;
	};
	wsSock.onmessage = evt => {
		try {
			let obj = JSON.parse(evt.data);
			if (typeof obj.type != "string" || obj.data == null) {
				throw new Exception("Message has invalid shape:", obj);
			}

			let handler = handlers[obj.type];
			if (handler == null) {
				console.warn("Got message of unhandled type:", obj.type);
				return;
			}

			handler(obj.data);
		} catch (err) {
			console.error("Failed to handle message '" + evt.data + "':", err)
		}
	};
}

function wsapi(t, data) {
	if (!wsConnected) {
		return;
	}

	wsSock.send(JSON.stringify({
		type: t,
		data: data,
	}));
}
