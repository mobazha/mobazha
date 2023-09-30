/**
 * Create a proxy of the Backbone Model which maps direct access of attributes
 * to the get/set interface that Backbone normally provides. This allows for
 * in-template binding to Backbone model attributes easily.
 *
 * In the case of ambiguity, say an attribute called "completed" and a method
 * called "completed". The method takes priority (so as not to break existing functionality),
 * however as a backup the attribute can be accessed with a prefix (conflictPrefix option),
 * e.g. model.$completed
 *
 * To avoid interference, it's stored under the Model property `_vuebackbone_proxy`. This proxy
 * is only really intended to be used by Vue templates.
 */

/**
 * Attach proxy getter/setter for a model attribute
 */
function proxyModelAttribute(proxy, model, attr, conflictPrefix) {
	let getter = model.get.bind(model, attr);
	let setter = model.set.bind(model, attr);

	// If there's a conflict with a function from the model, add the attribute with the prefix
	let safeAttr = proxy[attr] ? conflictPrefix + attr : attr;

	Object.defineProperty(proxy, safeAttr, {
		enumerable: true,
		get: getter,
		set: setter
	});
}

export default function(model, conflictPrefix) {
	let proxy = {};

	// Attach bound version of all the model functions to the proxy
	// these functions are readonly on the proxy and any future changes
	// to the model functions won't be reflected in the Vue proxy
	for (let key in model) {
		if (typeof model[key] === "function" && key !== "constructor") {
			var bndFunc = model[key].bind(model);
			Object.defineProperty(proxy, key, { value: bndFunc });
		}
	}

	// Attach getter/setters for the model attributes.
	Object.keys(model.attributes).forEach(attr => {
		proxyModelAttribute(proxy, model, attr, conflictPrefix);
	});
	if (!proxy.id) {
		// sometimes ID is a field in the model (in which case it'll be proxied already)
		Object.defineProperty(proxy, "id", {
			get: function() {
				return model.id;
			}
		});
	}

	// Attach link back to original model
	Object.defineProperty(proxy, "_vuebackbone_original", { value: model });

	return proxy;
}
