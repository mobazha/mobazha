import Moderators from './moderators/Moderators.vue';

const modules = import.meta.globEager('./*.vue')
const map = {}
Object.keys(modules).forEach(file => {
  const modulesName = file.replace('./', '').replace('.vue', '')
  map[modulesName] = modules[file].default
})

map['Moderators'] = Moderators;

const globalComponents = {
  ...map,
}
export default globalComponents
