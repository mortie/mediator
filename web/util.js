async function api(method, path, body = null) {
	let options = {method};
	if (body != null) {
		options.body = JSON.stringify(body);
	}

	let json;
	try {
		let resp = await fetch("/api/" + path, options).then(r => r.text());
		json = JSON.parse(resp);
	} catch (err) {
		alert(err.toString());
		throw err;
	}

	if (json.error != null) {
		alert(json.error);
		throw new Error(json.error);
	}

	return json;
}

function html(name, attrs, children) {
	if (!(children instanceof Array)) {
		children = [children];
	}

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
		if (typeof child == "string") {
			el.appendChild(document.createTextNode(child));
		} else {
			el.appendChild(child);
		}
	}

	return el;
}

function clearElement(el) {
	while (el.firstChild) {
		el.removeChild(el.firstChild);
	}
}

function renderToElement(el, children) {
	if (!(children instanceof Array)) {
		children = [children];
	}

	clearElement(el);
	for (let child of children) {
		el.appendChild(child);
	}
}
