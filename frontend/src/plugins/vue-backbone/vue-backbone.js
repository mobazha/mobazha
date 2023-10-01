import modelProxy from "./model-proxy.js";
import collectionProxy from "./collection-proxy.js";

/**
 * Default values for the possible options passed in Vue.use
 */
var opts = {
	proxies: true,
	conflictPrefix: "$",
	addComputed: true,
	dataPrefix: "_",
	simpleCollectionProxy: false
};

/**
 * Created a VueBackbone Proxy object which can be
 * accessed from the original Backbone Object
 */
function vueBackboneProxy(bb) {
	if (opts.proxies && !bb._vuebackbone_proxy) {
		if (bb.models) {
			bb.each(vueBackboneProxy);
			bb._vuebackbone_proxy = collectionProxy(
				bb,
				opts.simpleCollectionProxy
			);
		} else {
			bb._vuebackbone_proxy = modelProxy(bb, opts.conflictPrefix);
		}
	}
}

/**
 * Functions to retrieve the underlying POJO
 * beneath the Backbone objects
 */

function rawSrcModel(model) {
	return model.toJSON2();
}

function rawSrcCollection(collection) {
	return collection.map(rawSrcModel);
}

function rawSrc(bb) {
	return bb.models ? rawSrcCollection(bb) : rawSrcModel(bb);
}

/**
 * When Proxies are enabled, the computed value is the most
 * practical way to access the proxy (functionality and data together).
 * However without proxies, the raw data should be accessible
 * via the `bb` options key directly, and the instance (functionality)
 * will be accessible via the vm.$bb[key] property.
 */
function getDataKey(key) {
	return opts.addComputed && opts.proxies ? opts.dataPrefix + key : key;
}

/**
 * Setup handlers for Backbone events, so that Vue keeps sync.
 * Also ensure Models are mapped.
 */

function bindCollectionToVue(vm, key, ctx, bb) {
	// Handle mapping of models for Vue proxy
	if (opts.proxies) {
		bb.on("add", vueBackboneProxy); // map new models
		ctx.onreset = () => bb.each(vueBackboneProxy);
		bb.on("reset", ctx.onreset); // map complete reset
	}

	// Changes to collection array will require a full reset (for reactivity)
	ctx.onchange = () => {
		vm.$data[getDataKey(key)] = rawSrcCollection(bb);
		// Proxy array isn't by reference, so it needs to be updated
		// (this is less costly than recreating it)
		if (opts.proxies) {
			var proxy = bb._vuebackbone_proxy;
			proxy.length = 0; // truncate first
			bb.forEach(
				(entry, index) => (proxy[index] = entry._vuebackbone_proxy)
			);
		}
	};
	bb.on("reset sort remove add", ctx.onchange);
}

/**
 * As VueBackbone can't support reactivity on new attributes added to a Backbone
 * Model, there's a safety with warning for it.
 */
function bindModelToVue(ctx, bb) {
	ctx.onchange = () => {
		// Test for new attribute
		if (bb.keys().length > Object.keys(bb._previousAttributes).length) {
			// Not an error, as it may be the case this attribute is not needed for Vue at all
			console.warn(
				"VueBackbone: Adding new Model attributes after binding is not supported, provide defaults for all properties"
			);
		}
	};

	bb.on("change", ctx.onchange);
}

function bindBBToVue(vm, key) {
	var ctx = vm._vuebackbone[key],
		bb = ctx.bb;

	bb.models ? bindCollectionToVue(vm, key, ctx, bb) : bindModelToVue(ctx, bb);
}

/**
 * Cleanup if the Backbone link is changed, or if the Vue is destroyed
 */
function unbindBBFromVue(vm, key) {
	var ctx = vm._vuebackbone[key];

	if (ctx) {
		ctx.bb.off(null, ctx.onchange);
		ctx.onreset && ctx.bb.off(null, ctx.onreset);

		// The VueBackbone Proxy could be deleted at this
		// point, and the handler to proxy new models, but
		// this would cause problems if multiple
		// Vue objects used the same Backbone model/collection

		//ctx.bb.off(null, vueBackboneProxy);
		//delete ctx.bb._vuebackbone_proxy;
	}
}

/**
 * Update Vue data object, at this point it will already be a function (not a hash)
 * This will make the underlying source of the collection/model reactive.
 */
function extendData(vm, key) {
	var origDataFn = vm.$options.data,
		ctx = vm._vuebackbone[key],
		value = rawSrc(ctx.bb),
		dataKey = getDataKey(key);

	vm.$options.data = function() {
		let data = {},
			origData = origDataFn ? origDataFn.apply(this, arguments) : {};

		if (origData.hasOwnProperty(key)) {
			throw `VueBackbone: Property '${key}' mustn't exist within the Vue data already`;
		}
		// if (origData.hasOwnProperty(dataKey)) {
		// 	throw `VueBackbone: Property '${dataKey}' mustn't exist within the Vue data already`;
		// }
		// shallow copy (just in case)
		Object.keys(origData).forEach(attr => (data[attr] = origData[attr]));
		data[dataKey] = value;
		return data;
	};
}

/**
 * In the case proxies are disabled or computed accessor,
 * the Backbone instance is added to vm.$bb[key]
 *
 * Instance access will trigger, this._key (reactive) access,
 * which means any computed values recompute.
 * In the case of Collections, the reason this is needed is that calculations in the
 * collection can work off the internal models arrays, which isn't the same as the rawSrc one
 * For Models, this access is important in the case the full model object is replaced,
 * it will ensure the computed value recomputes.
 */
function extendVm(vm, key) {
	var ctx = vm._vuebackbone[key],
		dataKey = getDataKey(key);

	vm.$bb = vm.$bb || {};
	Object.defineProperty(vm.$bb, key, {
		get() {
			let access = vm.$data[dataKey]; // eslint-disable-line no-unused-vars
			return ctx.bb;
		},
		set(bb) {
			unbindBBFromVue(vm, key);
			ctx.bb = bb;
			vm.$data[dataKey] = rawSrc(bb);
			bindBBToVue(vm, key);
		}
	});
}

/**
 * Update Vue computed functions, this will provide a handy accessor (key)
 * for mapped models of a collection, or the mapped model directly.
 *
 * Computed (this.key) access will trigger, this._key (reactive) access,
 * which means any computed values recompute.
 * In the case of Collections, the reason this is needed is that calculations in the
 * collection can work off the internal models arrays, which isn't the same as the rawSrc one
 * For Models, this access is important in the case the full model object is replaced,
 * it will ensure the computed value recomputes.
 */
function extendComputed(vm, key) {
	var ctx = vm._vuebackbone[key],
		dataKey = getDataKey(key),
		o = vm.$options;

	o.computed = o.computed || {};

	// In the case of conflict, don't add it
	// if (!o.computed[key]) {
		o.computed[key] = {
			get() {
				let access = vm.$data[dataKey]; // eslint-disable-line no-unused-vars
				return ctx.bb._vuebackbone_proxy;
			},
			set(bb) {
				unbindBBFromVue(vm, key);
				vueBackboneProxy(bb);
				ctx.bb = bb;
				vm.$data[dataKey] = rawSrc(bb);
				bindBBToVue(vm, key);
			}
		};
	// } else {
	// 	console.warn(
	// 		`VueBackbone: Generated computed function '${key}' already exists within the Vue computed functions`
	// 	);
	// }
}

/**
 * Setup Vue and BB instance during Vue creation.
 * At this point the validation/normalization has
 * occurred.
 */
function initBBAndVue(vm, key, bb, prop) {
	vm._vuebackbone[key] = { bb: bb };

	vueBackboneProxy(bb);
	if (!prop) {
		extendData(vm, key);
		if (opts.addComputed && opts.proxies) {
			extendComputed(vm, key);
		} else {
			extendVm(vm, key);
		}
	}
	bindBBToVue(vm, key);
}

/**
 * Vue Mixin with Global Handlers
 */
let vueBackboneMixin = {
	beforeCreate() {
		var vm = this,
			bbopts = vm.$props.bb;
		if (bbopts) {
			if (typeof bbopts !== "function") {
				throw `VueBackbone: 'bb' initialization option must be a function`;
			}
			bbopts = bbopts(); // remember, it's a function
			vm._vuebackbone = {};

			Object.keys(bbopts).forEach(key => {
				var bb = bbopts[key],
					prop = false;

				// Detect Property
				if (bb.prop === true) {
					if (!vm.$options.propsData || !vm.$options.propsData[key]) {
						throw `VueBackbone: Missing Backbone object in Vue prop '${key}'`;
					}
					bb = vm.$options.propsData[key];
					prop = true;
				}

				// If Proxy, retrieve original instance
				bb = bb._vuebackbone_original || bb;

				// Detect Model or Collection
				if (bb.on && (bb.attributes || bb.models)) {
					initBBAndVue(vm, key, bb, prop);
				} else {
					throw `VueBackbone: Unrecognized Backbone object in Vue instantiation (${key}), must be a Collection or Model`;
				}
			});
		}
	},
	unmounted: function() {
		let vm = this,
			ctx = vm._vuebackbone;
		if (ctx) {
			Object.keys(ctx).forEach(key => unbindBBFromVue(vm, key));
		}
	}
};

/**
 * Maps an individual Backbone model, or an array of them, or an falsy value.
 * @returns either a hash of raw attributes, or the vue model proxy
 */
export function mapBBModels(func) {
	if (opts.proxies) {
		return function() {
			let models = func.apply(this, arguments);
			return (
				(models &&
					(models._vuebackbone_proxy ||
						models.map(m => m._vuebackbone_proxy))) ||
				models
			);
		};
	} else {
		return function() {
			let models = func.apply(this, arguments);
			return (
				(models &&
					(models.attributes || models.map(m => m.attributes))) ||
				models
			);
		};
	}
}

export function install(Vue, options) {
	for (let key in options) {
		if (options.hasOwnProperty(key)) {
			opts[key] = options[key];
		}
	}
	Vue.mixin(vueBackboneMixin);
}

export function original(bb) {
	return bb._vuebackbone_original || bb;
}

export default {
	install: install,
	mapBBModels: mapBBModels,
	original: original
};
