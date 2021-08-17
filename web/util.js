async function api(method, path, body = null) {
	let options = {method};
	if (body != null) {
		options.body = JSON.stringify(body);
	}

	let resp = await fetch("/api/" + path, options).then(r => r.text());
	let json = JSON.parse(resp);

	if (json.error != null) {
		throw new Error(json.error);
	}

	return json;
}

function html(name, attrs, children) {
	if (name == "text") {
		return document.createTextNode(attrs);
	}

	let el = document.createElement(name);
	for (let key in attrs) {
		if (!attrs.hasOwnProperty(key)) {
			continue;
		}

		el.setAttribute(key, attrs[key]);
	}

	for (let child of children) {
		el.appendChild(child);
	}

	return el;
}

function clearElement(el) {
	while (el.firstChild) {
		el.removeChild(el.firstChild);
	}
}
