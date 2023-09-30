/**
 * Create a proxy of the Backbone Collection which maps direct access of underlying
 * array of Model proxies to the functional interface that Backbone normally provides. 
 * This allows for in-template looping/access to Backbone model proxies.
 *
 * In the case of ambiguity, say a function exists on both the Array.prototype and in
 * Backbone. In general Backbone functionality is favoured, but there is a list of Array
 * functions which will be kept.
 *
 * In addition, common Backbone collection functions which return an array of models, or a single
 * one, have the returned Model(s) mapped to their proxies. Just a convenience.
 *
 * To avoid interference, it's stored under the Model property `_vuebackbone_proxy`. This proxy
 * is only really intended to be used by Vue templates.
 *
 * Generate documentation using: https://jsfiddle.net/fLcn09eb/3/
 */

const arrayPriority = [
	"slice",
	"forEach",
	"map",
	"reduce",
	"reduceRight",
	"find",
	"filter",
	"every",
	"some",
	"indexOf",
	"lastIndexOf",
	"findIndex"
],
	returnsModels = [
		"pop",
		"shift",
		"remove",
		"get",
		"at",
		"where",
		"findWhere",
		"reject",
		"sortBy",
		"shuffle",
		"toArray",
		"detect",
		"select",
		"first",
		"head",
		"take",
		"rest",
		"tail",
		"drop",
		"initial",
		"last",
		"without"
	];

function checkForReturnsModels(func, key) {
	if (returnsModels.indexOf(key) > -1) {
		return function() {
			let models = func.apply(this, arguments);
			return (
				(models &&
					(models._vuebackbone_proxy ||
						models.map(m => m._vuebackbone_proxy))) ||
				models
			);
		};
	}
	return func;
}

export default function(collection, simple) {
	let proxy = collection.map(model => model._vuebackbone_proxy);

	// Attach bound version of all the collection functions to the proxy
	// these functions are readonly on the proxy and any future changes
	// to the collection functions won't be reflected in the Vue proxy
	for (let key in collection) {
		if (
			typeof collection[key] === "function" &&
			key !== "constructor" &&
			(simple || arrayPriority.indexOf(key) === -1)
		) {
			let bndFunc = collection[key].bind(collection);
			if (!simple) {
				bndFunc = checkForReturnsModels(bndFunc, key);
			}
			Object.defineProperty(proxy, key, { value: bndFunc });
		}
	}

	// Attach link back to original model
	Object.defineProperty(proxy, "_vuebackbone_original", {
		value: collection
	});

	return proxy;
}
